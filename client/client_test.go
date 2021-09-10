package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Discovery/msg"
)

func testfinder(group, name string) (map[string]struct{}, error) {
	result := make(map[string]struct{})
	result["127.0.0.1:9234"] = struct{}{}
	result["127.0.0.1:9235"] = struct{}{}
	return result, nil
}
func Test_Client1(t *testing.T) {
	NewDiscoveryClient(&ClientConfig{
		VerifyData:       "test",
		DiscoverFunction: testfinder,
	}, "testgroup", "testclient1", "default", "discovery")
	rch, _ := NoticeRpcChanges("testgroup.testclient2")
	wch, _ := NoticeWebChanges("testgroup.testclient2")
	go func() {
		for {
			select {
			case <-rch:
				appreginfo := GetRpcInfos("testgroup.testclient2")
				log.Info(nil, "rpc:", appreginfo)
			case <-wch:
				appreginfo := GetWebInfos("testgroup.testclient2")
				log.Info(nil, "web:", appreginfo)
			}
		}
	}()
	time.Sleep(3 * time.Second)
	fmt.Println("register start")
	RegisterSelf(&msg.RegInfo{
		RpcPort: 9000,
		WebPort: 8000,
	})
	fmt.Println("register end")
	//time.Sleep(time.Second * 10)
	//for i := 0; i < 10; i++ {
	//        fmt.Println()
	//}
	//fmt.Println("unregister start")
	//UnRegisterSelf()
	//fmt.Println("unregister end")
	select {}
}
func Test_Client2(t *testing.T) {
	NewDiscoveryClient(&ClientConfig{
		VerifyData:       "test",
		DiscoverFunction: testfinder,
	}, "testgroup", "testclient2", "default", "discovery")
	rch, _ := NoticeRpcChanges("testgroup.testclient1")
	wch, _ := NoticeWebChanges("testgroup.testclient1")
	go func() {
		for {
			select {
			case <-rch:
				appreginfo := GetRpcInfos("testgroup.testclient1")
				log.Info(nil, "rpc:", appreginfo)
			case <-wch:
				appreginfo := GetWebInfos("testgroup.testclient1")
				log.Info(nil, "web:", appreginfo)
			}
		}
	}()
	time.Sleep(3 * time.Second)
	fmt.Println("register start")
	RegisterSelf(&msg.RegInfo{
		RpcPort: 9001,
		WebPort: 8001,
	})
	fmt.Println("register end")
	//time.Sleep(time.Second * 10)
	//for i := 0; i < 10; i++ {
	//        fmt.Println()
	//}
	//fmt.Println("unregister start")
	//UnRegisterSelf()
	//fmt.Println("unregister end")
	select {}
}
