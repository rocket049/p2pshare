# 基于 libp2p 的文件共享和聊天程序

启动命令为：`./p2pshare -name <名字>`

启动后首先要求输入密码，对于第一次使用的名字，该密码将会用于设置密码，后续登录会验证密码。

如果忘记了密码，可以删除 `HOME/.config/p2pshare/name_priv.key`，然后重新运行命令输入密码，但是以后你的 peer-ID就会改变了。

连接成功后终端会显示下列信息：

```
成功连接 bootstrap 节点: {QmdVoz8Y6QfKxvQ7nuC37JduuoAekeYDnzL46mBKa42XNM: [/ip4/148.70.58.15/tcp/4001]}
功能：使用libp2p共享文件和聊天。
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
	本节点的共享文件保存路径为： /home/xxxx/sharefiles
> 

```