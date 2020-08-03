package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gitee.com/rocket049/mycrypto"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	discovery "github.com/libp2p/go-libp2p-discovery"

	dhthost "github.com/libp2p/go-libp2p-core/host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	multiaddr "github.com/multiformats/go-multiaddr"

	"github.com/ipfs/go-log"
	"github.com/nsf/termbox-go"
)

var logger = log.Logger("p2pShare")

var talkStream network.Stream
var denyList = []string{}

var helpMsg = `功能：使用libp2p共享文件和聊天。
启动：
	./p2pshare -name <名字>
Commands:
	find <keyword>  -- 从网络中查找文件，返回搜索结果"p2p-ID:path/to/file"
	get <p2p-ID:path/to/file>  -- 从对方节点下载文件
	search <key>  -- 搜索名字包含key的用户
	whois <名字>  -- 搜索该名字对应的 p2p-ID
	talk <p2p-ID>  -- 和对方建立聊天连接
	say <something>  -- 向talk连接的对方发送聊天信息
	msg <somgthing>  -- 发送公共信息
	deny <p2p-ID>  -- 拒绝接受对方发出的公共信息
	msgto <p2p-ID> <something>  -- 向 p2p-ID 节点发送聊天信息
	本节点的共享文件保存路径为：`

func handleStream(stream network.Stream) {
	//logger.Info("Got a new stream!")

	// Create a buffer stream for non blocking read and write.
	//rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))

	go echoData(stream)

	// 'stream' will stay open until you close it (or the other side closes it).
}

func echoData(stream network.Stream) {
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	str, err := rw.ReadString('\n')
	if err != nil {
		stream.Close()
		return
	} //else {
	//fmt.Printf("\x1b[32m%s : %s\x1b[0m> ", stream.Conn().RemotePeer().String(), str)
	//}
	args := strings.SplitN(strings.TrimSpace(str), " ", 2)
	switch args[0] {
	case "find":
		res, err := searchfile(sharePath, strings.TrimSpace(args[1]))
		if err != nil {
			panic(err)
		}
		for i := range res {
			rw.WriteString(stream.Conn().LocalPeer().String() + ":" + res[i] + "\n")
		}

	case "get":
		//download file
		err = sendfile(stream, args[1])
		if err == nil {
			fmt.Println("send successful")
		} else {
			fmt.Println("send fail:", err)
		}

	case "whois":
		if args[1] == Name {
			rw.WriteString(fmt.Sprintf("%s -> %s\n", stream.Conn().LocalPeer().String(), Name))
		}
	case "search":
		if strings.Contains(Name, args[1]) {
			rw.WriteString(fmt.Sprintf("%s -> %s\n", stream.Conn().LocalPeer().String(), Name))
		}
	case "msg":
		if containName(denyList, stream.Conn().RemotePeer().String()) == false {
			fmt.Printf("\x1b[32m%s : %s\x1b[0m\n> ", stream.Conn().RemotePeer().String(), args[1])
		}

		stream.Close()
	case "talk":
		if talkStream != nil {
			talkStream.Close()
		}
		talkStream = stream
		fmt.Printf("\x1b[32mtalk connected : %s\x1b[0m> ", args[1])
		trw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
		go readData(trw)
		return

	default:
		rw.WriteString("Unknown command:" + str)

	}
	rw.Flush()
	stream.Close()
}

func containName(v []string, name string) bool {
	for i := range v {
		if v[i] == name {
			return true
		}
	}
	return false
}

func readData(rw *bufio.ReadWriter) {
	for {
		str, _ := rw.ReadString('\n')
		if str == "" {
			return
		}
		if str != "\n" {
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}

	}
}

func search(stream network.Stream, key string) {

	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	rw.WriteString("find " + key + "\n")
	rw.Flush()
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			//fmt.Println("Error reading from buffer")
			break
		}

		if str == "" {
			break
		}
		if str != "\n" {
			// Green console colour: 	\x1b[32m
			// Reset console colour: 	\x1b[0m
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}

	}
}

var Name string

func main() {
	log.SetAllLoggers(log.LevelError)
	log.SetLogLevel("p2pShare", "warn")
	help := flag.Bool("h", false, "Display Help")
	pub := flag.Bool("pub", false, "公开公网地址")
	nocrypt := flag.Bool("nocrypt", false, "是否对私有密钥加密")
	flag.StringVar(&Name, "name", "", "set user name")
	bootstrap := flag.String("bootstrap", "/ip4/148.70.58.15/tcp/4001/p2p/QmdVoz8Y6QfKxvQ7nuC37JduuoAekeYDnzL46mBKa42XNM", "bootstrap node address")

	config, err := ParseFlags()
	if err != nil {
		panic(err)
	}
	//不同名字，不同 private key
	keyPath = filepath.Join(keyPath, Name+"_priv.key")

	if *help {
		fmt.Println("This program demonstrates a simple p2p file share application using libp2p")
		flag.PrintDefaults()
		return
	}

	ctx := context.Background()

	// libp2p.New constructs a new libp2p Host. Other options can be added
	// here.
	var sourceMultiAddr multiaddr.Multiaddr
	var port int = 0
	if *pub {
		port = 4001
	}
	sourceMultiAddr, _ = multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", port))
	privkey := getPrivKey(keyPath, *nocrypt)

	host, err := libp2p.New(ctx,
		libp2p.ListenAddrs([]multiaddr.Multiaddr{sourceMultiAddr}...),
		libp2p.Identity(privkey),
		libp2p.DefaultSecurity,
	)
	if err != nil {
		panic(err)
	}
	fmt.Println("Host created. We are:", host.ID())

	for _, addr1 := range host.Addrs() {
		if strings.Contains(addr1.String(), "127.") || strings.Contains(addr1.String(), "::1") {
			continue
		}
		maddr := fmt.Sprintf("%s/p2p/%s", addr1.String(), host.ID().String())
		fmt.Println("Listen:", maddr)
	}

	// Set a function as stream handler. This function is called when a peer
	// initiates a connection and starts a stream with this peer.
	host.SetStreamHandler(protocol.ID(config.ProtocolID), handleStream)

	// We use a rendezvous point "meet me here" to announce our location.
	// This is like telling your friends to meet you at the Eiffel Tower.
	logger.Info("Announcing ourselves...")

	// Start a DHT, for use in peer discovery. We can't just make a new DHT
	// client because we want each peer to maintain its own local copy of the
	// DHT, so that the bootstrapping node of the DHT can go down without
	// inhibiting future peer discovery.

	kademliaDHT, err := dht.New(ctx, host, dht.Mode(dht.ModeServer))
	if err != nil {
		panic(err)
	}

	err = nil
	maddrs := []string{*bootstrap}

	var peerinfos = []peer.AddrInfo{}

	var wg sync.WaitGroup
	mas, _ := StringsToAddrs(maddrs)

	for _, peerAddr := range mas {

		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)
		host.Peerstore().AddAddrs(peerinfo.ID, peerinfo.Addrs, peerstore.PermanentAddrTTL)
		peerinfos = append(peerinfos, *peerinfo)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, *peerinfo); err == nil {
				fmt.Println("成功连接 bootstrap 节点:", *peerinfo)
			}
		}()
	}
	wg.Wait()

	// Bootstrap the DHT. In the default configuration, this spawns a Background
	// thread that will refresh the peer table every five minutes.
	logger.Debug("Bootstrapping the DHT")
	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		panic(err)
	}
	routingDiscovery := discovery.NewRoutingDiscovery(kademliaDHT)
	d, _ := routingDiscovery.Advertise(ctx, config.RendezvousString)
	go func() {
		t := time.NewTicker(d)
		for {
			<-t.C
			routingDiscovery.Advertise(ctx, config.RendezvousString)
		}
	}()

	//discovery.Advertise(ctx, routingDiscovery, config.RendezvousString)
	logger.Info("Successfully announced!")

	fmt.Println(helpMsg, sharePath)

	stdReader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("> ")
		sendData, err := stdReader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from stdin")
			panic(err)
		}
		sendData = strings.TrimSpace(sendData)
		args := strings.SplitN(sendData, " ", 2)
		if len(args) != 2 {
			if strings.TrimSpace(sendData) == "exit" {
				break
			}
			if len(sendData) > 0 {
				fmt.Println("error command")
			}

			continue
		}
		switch args[0] {
		case "find":
			searchDht(host, routingDiscovery, ctx, &config, args[1])
		case "get":
			vs := strings.SplitN(args[1], ":", 2)
			if len(vs) != 2 {
				fmt.Println("error get command")
				continue
			}
			id1, err := peer.Decode(vs[0])
			if err != nil {
				fmt.Println(vs[0], err.Error())
				continue
			}
			stream1, err := host.NewStream(ctx, id1, protocol.ID(config.ProtocolID))
			if err != nil {
				logger.Warning("connect " + id1.String() + ":" + err.Error())
				continue
			}
			fmt.Println("connected:", vs[0])
			_, err = stream1.Write([]byte("get " + vs[1] + "\n"))
			if err != nil {
				logger.Warning(err.Error())
				stream1.Close()
				continue
			}
			err = recvfile(stream1, filepath.Base(vs[1]))
			if err == nil {
				fmt.Println("recv successful")
			} else {
				fmt.Println("recv fail:", err)
			}
		case "talk":
			id1, err := peer.Decode(args[1])
			if err != nil {
				fmt.Println(args[1], err.Error())
				continue
			}
			stream1, err := host.NewStream(ctx, id1, protocol.ID(config.ProtocolID))
			if err != nil {
				logger.Warning("connect " + id1.String() + ":" + err.Error())
				continue
			}
			if talkStream != nil {
				talkStream.Close()
			}
			talkStream = stream1
			tw := bufio.NewWriter(talkStream)
			tw.WriteString(fmt.Sprintf("talk %s\n", stream1.Conn().LocalPeer().String()))
			tw.Flush()
			fmt.Println("talk connected:", args[1])
			trw := bufio.NewReadWriter(bufio.NewReader(stream1), bufio.NewWriter(stream1))
			go readData(trw)
		case "say":
			if talkStream == nil {
				fmt.Println("没有链接聊天对象，请先用 <talk ID> 命令建立连接。")
				continue
			}
			tw := bufio.NewWriter(talkStream)
			tw.WriteString(fmt.Sprintf("%s\n", args[1]))
			tw.Flush()
		case "whois":
			queryDht(host, routingDiscovery, ctx, &config, sendData+"\n")
		case "search":
			queryDht(host, routingDiscovery, ctx, &config, sendData+"\n")
		case "deny":
			if containName(denyList, args[1]) == false {
				denyList = append(denyList, args[1])
			}
		case "msg":
			queryDht(host, routingDiscovery, ctx, &config, fmt.Sprintf("msg (%s)%s\n", Name, args[1]))
		case "msgto":
			msgs := strings.SplitN(args[1], " ", 2)
			msgTo(ctx, host, strings.TrimSpace(msgs[0]), strings.TrimSpace(msgs[1]), &config)

		default:
			fmt.Println("unknown command")
		}
		//end case

	}
}

func msgTo(ctx context.Context, host dhthost.Host, idStr string, msg string, config *Config) error {
	id1, err := peer.Decode(idStr)
	if err != nil {
		return err
	}
	stream1, err := host.NewStream(ctx, id1, protocol.ID(config.ProtocolID))
	if err != nil {
		return err
	}
	w := bufio.NewWriter(stream1)
	_, err = w.WriteString(fmt.Sprintf("msg (%s)%s\n", Name, msg))
	w.Flush()
	return err
}

func searchDht(host dhthost.Host, routingDiscovery *discovery.RoutingDiscovery, ctx context.Context, config *Config, key string) {
	//logger.Debug("Searching for other peers...")
	peerChan, err := routingDiscovery.FindPeers(ctx, config.RendezvousString)
	if err != nil {
		panic(err)
	}

	for peer1 := range peerChan {
		if peer1.ID == host.ID() {
			continue
		}

		stream, err := host.NewStream(ctx, peer1.ID, protocol.ID(config.ProtocolID))

		if err != nil {

			continue
		} else {

			go search(stream, key)
		}

	}
	fmt.Println("search end:", key)
}

func sendCmdShowEcho(stream network.Stream, cmdstr string) {
	rw := bufio.NewReadWriter(bufio.NewReader(stream), bufio.NewWriter(stream))
	rw.WriteString(cmdstr)
	rw.Flush()
	//fmt.Println("send:", cmdstr)
	for {
		str, err := rw.ReadString('\n')
		if err != nil {
			break
		} else {
			fmt.Printf("\x1b[32m%s\x1b[0m> ", str)
		}
	}

}

func queryDht(host dhthost.Host, routingDiscovery *discovery.RoutingDiscovery, ctx context.Context, config *Config, cmdstr string) {
	//logger.Debug("Searching for other peers...")
	peerChan, err := routingDiscovery.FindPeers(ctx, config.RendezvousString)
	if err != nil {
		panic(err)
	}

	for peer1 := range peerChan {
		if peer1.ID == host.ID() {
			continue
		}

		stream, err := host.NewStream(ctx, peer1.ID, protocol.ID(config.ProtocolID))

		if err != nil {
			logger.Warning("Connection failed:", err)
			continue
		} else {

			go sendCmdShowEcho(stream, cmdstr)
		}

		//logger.Info("Connected to:", peer)
	}
	//fmt.Println("query end:", cmdstr)
}

func getPrivKey(filename string, nocrypt bool) (prvkey crypto.PrivKey) {
	crypt := !nocrypt

	if crypt {
		pwd := getPass()
		data, err := mycrypto.CfbDecodeFromFile(filename, pwd)
		if err == nil {
			prvkey, err = crypto.UnmarshalPrivateKey(data)
			if err == nil {
				return
			} else {
				panic("unmarshal priv key:" + err.Error())
			}
		} else if err == mycrypto.ErrPasswordError {
			panic(err)
		}
		//fmt.Println("privkey:", err)
		// Creates a new RSA key pair for this host.
		prvkey, _, err = crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
		if err != nil {
			panic(err)
		}

		data, err = crypto.MarshalPrivateKey(prvkey)
		if err != nil {
			panic("marshal priv key:" + err.Error())
		}
		err = mycrypto.CfbEncodeToFile(data, filename, pwd)
		if err != nil {
			panic("save priv key:" + err.Error())
		}
	} else {
		data, err := ioutil.ReadFile(filename)
		if err == nil {
			prvkey, err = crypto.UnmarshalPrivateKey(data)
			if err == nil {
				return
			} else {
				panic("unmarshal priv key:" + err.Error())
			}
		}
		// Creates a new RSA key pair for this host.
		prvkey, _, err = crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, rand.Reader)
		if err != nil {
			panic(err)
		}

		data, err = crypto.MarshalPrivateKey(prvkey)
		if err != nil {
			panic("marshal priv key:" + err.Error())
		}
		err = ioutil.WriteFile(filename, data, 0644)
		if err != nil {
			panic("save priv key:" + err.Error())
		}
	}
	return
}

func getPass() string {
	data := bytes.NewBufferString("")
	termbox.Init()
	defer termbox.Close()
	fmt.Println("输入用户密码：")
	for {
		e := termbox.PollEvent()

		if e.Type == termbox.EventKey {
			if e.Key == 13 {
				break
			}
			data.WriteRune(e.Ch)
		}
	}
	data.WriteByte('\n')
	return data.String()
}
