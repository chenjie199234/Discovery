package sdk

import (
	"github.com/chenjie199234/Corelib/discovery"
)

func NewSdk(selfgroup, selfname, verifydata string) error {
	if e := discovery.NewDiscoveryClient(nil, selfgroup, selfname, verifydata, discovery.MakeDefaultFinder("default", "discovery", 1000)); e != nil {
		return e
	}
	return nil
}

func RegisterSelf(rpcport, webport int, webscheme string, addition []byte) {
	discovery.RegisterSelf(&discovery.RegMsg{
		WebScheme: webscheme,
		WebPort:   webport,
		RpcPort:   rpcport,
		Addition:  addition,
	})
}
