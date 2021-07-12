package sdk

import (
	"bytes"
	"context"
	"errors"
	"net"
	"sort"
	"sync/atomic"
	"time"
	"unsafe"

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
	if e := client.NewDiscoveryClient(nil, selfgroup, selfname, verifydata, func(manually chan struct{}, client *client.DiscoveryClient) {
		host := "discovery-service-headless.default"
		appname := "default.discovery"

		current := make([]string, 0)

		finder := func() {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Second))
			defer cancel()
			addrs, e := net.DefaultResolver.LookupHost(ctx, host)
			if e != nil {
				log.Error("[Discovery.sdk] dns resolve host:", host, "error:", e)
				return
			}
			if len(addrs) != 0 {
				sort.Strings(addrs)
				for i, addr := range addrs {
					addrs[i] = appname + ":" + addr + ":10000"
				}
			}
			different := false
			if len(current) != len(addrs) {
				different = true
			} else {
				for i, addr := range addrs {
					if addr != current[i] {
						different = true
						break
					}
				}
			}
			if different {
				current = addrs
				log.Info("[Discovery.sdk] dns resolve host:", host, "result:", current)
				client.UpdateDiscoveryServers(addrs)
			}
		}
		finder()
		tker := time.NewTicker(time.Second * 5)
		for {
			select {
			case <-tker.C:
				finder()
			case <-manually:
				finder()
			}
		}
	}); e != nil {
		return e
	}
	return nil
}
func RegRpc(rpcport int64) error {
	if rpcport <= 0 || rpcport > 65535 {
		return errors.New("[Discovery.sdk] rpcport out of range")
	}
	if regmsg != nil {
		if rpcport == regmsg.WebPort {
			return errors.New("[Discovery.sdk] rpcport and webport conflict")
		}
		regmsg.RpcPort = rpcport
		return nil
	} else {
		return errors.New("[Discovery.sdk] not inited")
	}
}
func RegWeb(webport int64, webscheme string) error {
	if webport <= 0 || webport > 65535 {
		return errors.New("[Discovery.sdk] webport out of range")
	}
	if regmsg != nil {
		if webport == regmsg.RpcPort {
			return errors.New("[Discovery.sdk] rpcport and webport conflict")
		}
		if webscheme != "http" && webscheme != "https" {
			return errors.New("[Discovery.sdk] webscheme unknown,must be http or https")
		}
		regmsg.WebPort = webport
		regmsg.WebScheme = webscheme
		return nil
	} else {
		return errors.New("[Discovery.sdk] not inited")
	}
}

func RegisterSelf(addition []byte) error {
	if regmsg != nil {
		if bytes.Contains(addition, []byte{'|'}) {
			return errors.New("[Discovery.sdk] addition data contains illegal character '|'")
		}
		regmsg.Addition = addition
		return client.RegisterSelf(regmsg)
	} else {
		return errors.New("[Discovery.sdk] not inited")
	}
}

func UnRegisterSelf() {
	client.UnRegisterSelf()
}
