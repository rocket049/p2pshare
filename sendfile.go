package main

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type FileHeader struct {
	Name  string
	Size  int64
	MTime time.Time
	Mode  os.FileMode
}

type FileData struct {
	Len  int32
	Data []byte
}

type VerifyTail struct {
	Md5 []byte
}

func sendfile(w io.WriteCloser, fn string) error {
	defer w.Close()
	filename := filepath.Join(sharePath, fn)
	info, err := os.Stat(filename)
	if err != nil {
		return err
	}
	header := FileHeader{Name: filepath.Base(filename), Size: info.Size(), MTime: info.ModTime(), Mode: info.Mode()}
	gobEnc := gob.NewEncoder(w)
	err = gobEnc.Encode(header)
	if err != nil {
		return err
	}
	fp, _ := os.Open(filename)
	hash := md5.New()
	data := make([]byte, 1024)
	for {
		n, err := fp.Read(data)
		if err != nil || n == 0 {
			break
		}
		hash.Write(data[:n])

		zdata, err := dataGzip(data[:n])
		if err != nil {
			return err
		}
		data1 := FileData{Len: int32(n), Data: zdata}
		err = gobEnc.Encode(data1)
		if err != nil {
			return err
		}
	}
	v := hash.Sum(nil)
	tail := VerifyTail{Md5: v}
	err = gobEnc.Encode(tail)
	return err
}

func recvfile(r io.Reader, filename string) error {
	var header FileHeader

	gobDec := gob.NewDecoder(r)
	err := gobDec.Decode(&header)
	if err != nil {
		return fmt.Errorf("decode header:%s", err.Error())
	}
	fp, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer fp.Close()
	hash := md5.New()
	var dataLen int64 = 0
	for {
		var data1 FileData
		err = gobDec.Decode(&data1)
		if err != nil {
			return fmt.Errorf("decode file data:%s", err.Error())
		}
		dataLen += int64(data1.Len)

		uzdata, err := dataGunzip(data1.Data)
		if err != nil {
			return fmt.Errorf("gunzip:%s", err.Error())
		}
		hash.Write(uzdata)
		fp.Write(uzdata)
		if dataLen == header.Size {
			//fmt.Println("recv success full")
			break
		}
		if dataLen > header.Size {
			return errors.New("file size error")
		}
	}
	var tail VerifyTail
	err = gobDec.Decode(&tail)
	if err != nil {
		return fmt.Errorf("decode tail:%s", err.Error())
	}
	v := hash.Sum(nil)
	if bytes.Compare(v, tail.Md5) != 0 {
		return errors.New("MD5 check error")
	}
	return nil
}

func dataGzip(data []byte) ([]byte, error) {
	buf := bytes.NewBufferString("")
	gzw, err := gzip.NewWriterLevel(buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	_, err = gzw.Write(data)
	if err != nil {
		return nil, err
	}
	gzw.Close()
	return buf.Bytes(), nil
}

func dataGunzip(data []byte) ([]byte, error) {
	buf := bytes.NewBuffer(data)
	gzr, err := gzip.NewReader(buf)
	if err != nil {
		return nil, err
	}
	out := bytes.NewBufferString("")
	_, err = out.ReadFrom(gzr)
	if err != nil {
		return nil, fmt.Errorf("Not EOF:%s", err.Error())
	}
	gzr.Close()
	return out.Bytes(), nil
}
