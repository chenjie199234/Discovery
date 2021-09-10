package sdk

import (
	"errors"
	"time"

	"github.com/chenjie199234/Discovery/client"

	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/rpc"
	"github.com/chenjie199234/Corelib/web"
)

func RpcDiscover(noticegroup, noticename string) func(string, string, <-chan struct{}) (map[string]*rpc.RegisterData, error) {
	var notice <-chan struct{}
	var e error
	for {
		if regmsg == nil {
			log.Error(nil, "[Discovery.sdk] not inited")
		} else {
			notice, e = client.NoticeRpcChanges(noticegroup + "." + noticename)
			if e == nil {
				break
			}
			log.Error(nil, "[Discovery.sdk] notice error:", e)
		}
		time.Sleep(time.Millisecond * 50)
	}
	return func(group, name string, manually <-chan struct{}) (map[string]*rpc.RegisterData, error) {
		select {
		case <-notice:
		case <-manually:
		}
		if noticegroup != group || noticename != name {
			log.Error(nil, "[Discovery.sdk] app conflict")
			return nil, errors.New("[Discovery.sdk] app conflict")
		}
		apps := client.GetRpcInfos(group + "." + name)
		result := make(map[string]*rpc.RegisterData, len(apps))
		for addr, app := range apps {
			result[addr] = &rpc.RegisterData{
				DServers: app.DiscoveryServerAddrs,
				Addition: app.Addition,
			}
		}
		return result, nil
	}
}
func WebDiscover(noticegroup, noticename string) func(string, string, <-chan struct{}) (map[string]*web.RegisterData, error) {
	var notice <-chan struct{}
	var e error
	for {
		if regmsg == nil {
			log.Error(nil, "[Discovery.sdk] not inited")
		} else {
			notice, e = client.NoticeWebChanges(noticegroup + "." + noticename)
			if e == nil {
				break
			}
			log.Error(nil, "[Discovery.sdk] notice error:", e)
		}
		time.Sleep(time.Millisecond * 50)
	}
	return func(group, name string, manually <-chan struct{}) (map[string]*web.RegisterData, error) {
		select {
		case <-notice:
		case <-manually:
		}
		if noticegroup != group || noticename != name {
			log.Error(nil, "[Discovery.sdk] app conflict")
			return nil, errors.New("[Discovery.sdk] app conflict")
		}
		apps := client.GetWebInfos(group + "." + name)
		result := make(map[string]*web.RegisterData, len(apps))
		for addr, app := range apps {
			result["http://"+addr] = &web.RegisterData{
				DServers: app.DiscoveryServerAddrs,
				Addition: app.Addition,
			}
		}
		return result, nil
	}
}
