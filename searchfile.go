package main

import (
	"container/list"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func searchfile(dirpath, key string) ([]string, error) {
	var res = []string{}
	dirList := list.New()
	dirList = dirList.Init()
	dirList.PushFront(dirpath)
	for {
		e := dirList.Front()
		d := e.Value.(string)
		infos, err := ioutil.ReadDir(d)
		if err != nil {
			return nil, err
		}
		for _, info := range infos {
			if info.IsDir() {
				dirList.PushBack(filepath.Join(d, info.Name()))
				continue
			} else if strings.Contains(info.Name(), key) {
				res = append(res, filepath.Join(strings.TrimLeft(d, sharePath), info.Name()))
			}
		}

		dirList.Remove(e)
		if dirList.Len() == 0 {
			break
		}
	}
	return res, nil
}

var sharePath string
var keyPath string

func init() {
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, "sharefiles")
	os.MkdirAll(p, os.ModePerm)
	sharePath = p

	p = filepath.Join(home, ".config", "p2pshare")
	os.MkdirAll(p, os.ModePerm)
	keyPath = p

}
