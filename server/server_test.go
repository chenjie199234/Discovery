package server

import (
	"testing"
	"time"

	"github.com/chenjie199234/Corelib/log"
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
			log.Info(instance.GetAppInfos())
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
			log.Info(instance.GetAppInfos())
		}
	}()
	instance.StartDiscoveryServer("127.0.0.1:9235")
}
