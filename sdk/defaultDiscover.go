package sdk

import (
	"time"

	"github.com/chenjie199234/Discovery/client"

	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/rpc"
	"github.com/chenjie199234/Corelib/web"
)

func DefaultRpcDiscover(group, name string, manually <-chan struct{}, c *rpc.RpcClient) {
	var notice chan struct{}
	var e error
	for {
		notice, e = client.NoticeRpcChanges(group + "." + name)
		if e != nil {
			log.Error("[rpc.client.defaultDiscover] register notice error:", e)
			time.Sleep(time.Millisecond * 100)
		} else {
			break
		}
	}

	for {
		<-notice
		addrs, additions := client.GetRpcInfos(group + "." + name)
		all := make(map[string]*rpc.RegisterData, len(addrs)+2)
		for addr := range addrs {
			all[addr] = &rpc.RegisterData{
				DServers: make(map[string]struct{}, len(addrs[addr])+2),
				Addition: additions[addr],
			}
			for _, dserver := range addrs[addr] {
				all[addr].DServers[dserver] = struct{}{}
			}
		}
		c.UpdateDiscovery(all)
	}
}

func DefaultWebDiscover(group, name string, manually <-chan struct{}, c *web.WebClient) {
	var notice chan struct{}
	var e error
	for {
		notice, e = client.NoticeWebChanges(group + "." + name)
		if e != nil {
			log.Error("[web.client.defaultDiscover] register notice error:", e)
			time.Sleep(time.Millisecond * 10)
		} else {
			break
		}
	}

	for {
		<-notice
		addrs, additions := client.GetWebInfos(group + "." + name)
		all := make(map[string]*web.RegisterData, len(addrs)+2)
		for addr := range addrs {
			all[addr] = &web.RegisterData{
				DServers: make(map[string]struct{}, len(addrs[addr])+2),
				Addition: additions[addr],
			}
			for _, dserver := range addrs[addr] {
				all[addr].DServers[dserver] = struct{}{}
			}
		}
		c.UpdateDiscovery(all)
	}
}
