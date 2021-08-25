package server

import (
	"fmt"
	"testing"
	"time"
)

func Test_Server1(t *testing.T) {
	instance, e := NewDiscoveryServer(&ServerConfig{
		HeartTimeout:           time.Second * 5,
		HeartPorbe:             time.Second * 2,
		SocketRBuf:             1024,
		SocketWBuf:             1024,
		GroupNum:               1,
		MaxMsgLen:              65535,
		MaxBufferedWriteMsgNum: 256,
		VerifyDatas:            []string{"test"},
	}, "default", "discovery")
	if e != nil {
		panic(e)
	}
	go func() {
		tker := time.NewTicker(time.Second)
		for {
			<-tker.C
			fmt.Println("client1:", instance.GetAppInfo("testgroup.testclient1")["testgroup.testclient1"])
			fmt.Println("client2:", instance.GetAppInfo("testgroup.testclient2")["testgroup.testclient2"])
		}
	}()
	instance.StartDiscoveryServer("127.0.0.1:9234")
}
func Test_Server2(t *testing.T) {
	instance, e := NewDiscoveryServer(&ServerConfig{
		HeartTimeout:           time.Second * 5,
		HeartPorbe:             time.Second * 2,
		SocketRBuf:             1024,
		SocketWBuf:             1024,
		GroupNum:               1,
		MaxMsgLen:              65535,
		MaxBufferedWriteMsgNum: 256,
		VerifyDatas:            []string{"test"},
	}, "default", "discovery")
	if e != nil {
		panic(e)
	}
	go func() {
		tker := time.NewTicker(time.Second)
		for {
			<-tker.C
			fmt.Println("client1:", instance.GetAppInfo("testgroup.testclient1")["testgroup.testclient1"])
			fmt.Println("client2:", instance.GetAppInfo("testgroup.testclient2")["testgroup.testclient2"])
		}
	}()
	instance.StartDiscoveryServer("127.0.0.1:9235")
}
