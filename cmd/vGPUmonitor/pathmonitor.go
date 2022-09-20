package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strings"
	"sync"

	vGPUmonitor "4pd.io/k8s-vgpu/cmd/vGPUmonitor/noderpc"
	"google.golang.org/grpc"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const containerpath = "/tmp/vgpu/containers"

type podusage struct {
	idstr string
	sr    *sharedRegionT
}

var lock sync.Mutex

func checkfiles(fpath string) (*sharedRegionT, error) {
	fmt.Println("Checking path", fpath)
	files, err := ioutil.ReadDir(fpath)
	if err != nil {
		return nil, err
	}
	if len(files) > 1 {
		return nil, errors.New("cache num not matched")
	}
	if len(files) == 0 {
		return nil, nil
	}
	for _, val := range files {
		strings.Contains(val.Name(), ".cache")
		cachefile := fpath + "/" + val.Name()
		nc := nvidiaCollector{
			cudevshrPath: cachefile,
			at:           nil,
		}
		sr, err := getvGPUMemoryInfo(&nc)
		if err != nil {
			fmt.Println("err=", err.Error())
		} else {
			//fmt.Println("sr=", sr.utilizationSwitch, sr.recentKernel, sr.priority)
			return sr, nil
		}
	}
	return nil, nil
}

func checkpodvalid(name string) bool {
	pods, err := clientset.CoreV1().Pods("").List(context.Background(), v1.ListOptions{})
	if err != nil {
		return true
	}
	for _, val := range pods.Items {
		if strings.Contains(name, string(val.UID)) {
			return true
		}
	}
	return false
}

func monitorpath() ([]podusage, error) {
	lock.Lock()
	defer lock.Unlock()
	srlist := []podusage{}
	files, err := ioutil.ReadDir(containerpath)
	if err != nil {
		return srlist, err
	}
	for _, val := range files {
		//fmt.Println("val=", val.Name())
		dirname := containerpath + "/" + val.Name()
		info, err1 := os.Stat(dirname)
		if err1 != nil || !checkpodvalid(info.Name()) {
			fmt.Println("removing" + dirname)
			err2 := os.RemoveAll(dirname)
			if err2 != nil {
				return srlist, err2
			}
		} else {
			fmt.Println(info.IsDir())
			sr, err2 := checkfiles(dirname)
			if err2 != nil {
				return srlist, err2
			}
			srlist = append(srlist, podusage{
				idstr: val.Name(),
				sr:    sr,
			})
		}
	}
	return srlist, nil
}

type server struct {
	vGPUmonitor.UnimplementedNodeVGPUInfoServer
}

func serveinfo(ch chan error) {
	s := grpc.NewServer()
	lis, err := net.Listen("tcp", ":9395")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	vGPUmonitor.RegisterNodeVGPUInfoServer(s, &server{})
	fmt.Println("server listening at", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	} /*
		for {
			val, err := monitorpath()
			if err != nil {
				ch <- err
				break
			}

			time.Sleep(time.Second * 10)
		}*/
}
