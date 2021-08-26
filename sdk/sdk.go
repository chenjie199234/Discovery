package sdk

import (
	"errors"
	"net"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/chenjie199234/Discovery/api"
	"github.com/chenjie199234/Discovery/client"
	"github.com/chenjie199234/Discovery/msg"

	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/util/common"
)

var regmsg *msg.RegInfo

func NewSdk(selfgroup, selfname, verifydata string) error {
	if e := common.NameCheck(selfname, false, true, false, true); e != nil {
		return errors.New("[Discovery.sdk] selfname:" + selfname + " check error:" + e.Error())
	}
	if e := common.NameCheck(selfgroup, false, true, false, true); e != nil {
		return errors.New("[Discovery.sdk] selfgroup:" + selfgroup + " check error:" + e.Error())
	}
	selfappname := selfgroup + "." + selfname
	if e := common.NameCheck(selfappname, true, true, false, true); e != nil {
		return errors.New("[Discovery.sdk] selfappname:" + selfappname + " check error:" + e.Error())
	}
	if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&regmsg)), nil, unsafe.Pointer(&msg.RegMsg{})) {
		return nil
	}
	if e := client.NewDiscoveryClient(&client.ClientConfig{
		VerifyData:       verifydata,
		DiscoverInterval: time.Second * 10,
		DiscoverFunction: func(group, name string) (map[string]struct{}, error) {
			result := make(map[string]struct{})
			addrs, e := net.LookupHost(name + "-service-headless" + "." + group)
			if e != nil {
				log.Error("[Discovery.sdk] get:", name+"-service-headless", "addrs error:", e)
				return nil, e
			}
			for _, addr := range addrs {
				result[addr] = struct{}{}
			}
			return result, nil
		},
	}, selfgroup, selfname, api.Group, api.Name); e != nil {
		return e
	}
	return nil
}

func RegRpc(rpcport int64) error {
	if rpcport <= 0 || rpcport > 65535 {
		return errors.New("[Discovery.sdk] rpc port out of range")
	}
	if regmsg == nil {
		return errors.New("[Discovery.sdk] not inited")
	}
	if rpcport == regmsg.WebPort {
		return errors.New("[Discovery.sdk] rpc port and web port conflict")
	}
	regmsg.RpcPort = rpcport
	return nil
}

func RegWeb(webport int64, webscheme string) error {
	if webport <= 0 || webport > 65535 {
		return errors.New("[Discovery.sdk] web port out of range")
	}
	if regmsg == nil {
		return errors.New("[Discovery.sdk] not inited")
	}
	if webport == regmsg.RpcPort {
		return errors.New("[Discovery.sdk] rpc port and web port conflict")
	}
	regmsg.WebPort = webport
	return nil
}

func RegisterSelf(addition []byte) error {
	if regmsg == nil {
		return errors.New("[Discovery.sdk] not inited")
	}
	regmsg.Addition = addition
	return client.RegisterSelf(regmsg)
}

func UnRegisterSelf() {
	client.UnRegisterSelf()
}
