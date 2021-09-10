package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/chenjie199234/Discovery/msg"

	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/stream"
	"github.com/chenjie199234/Corelib/util/common"
	"google.golang.org/protobuf/proto"
)

//return data's key is server's addr "ip:port"
type DiscoveryHandler func(group, name string) (map[string]struct{}, error)

func defaultDiscover(group, name string, client *DiscoveryClient) {
	notice := func() {
		client.mlker.Lock()
		for notice := range client.manualNotice {
			notice <- struct{}{}
			delete(client.manualNotice, notice)
		}
		client.mlker.Unlock()
	}
	var check []byte
	tker := time.NewTicker(client.c.DiscoverInterval)
	for {
		select {
		case <-tker.C:
		case <-client.manually:
		}
		all, e := client.c.DiscoverFunction(group, name)
		if e != nil {
			continue
		}
		d, _ := json.Marshal(all)
		if bytes.Equal(check, d) {
			notice()
			continue
		}
		check = d
		client.update(all)
		notice()
		tker.Reset(client.c.DiscoverInterval)
		for len(tker.C) > 0 {
			<-tker.C
		}
	}
}

type ClientConfig struct {
	ConnTimeout            time.Duration
	HeartTimeout           time.Duration
	HeartPorbe             time.Duration
	GroupNum               uint32
	SocketRBuf             uint32
	SocketWBuf             uint32
	MaxMsgLen              uint32
	MaxBufferedWriteMsgNum uint32
	VerifyData             string
	DiscoverFunction       DiscoveryHandler
	DiscoverInterval       time.Duration //min 1 second
}

func (c *ClientConfig) validate() {
	if c.ConnTimeout <= 0 {
		c.ConnTimeout = time.Millisecond * 500
	}
	if c.HeartTimeout <= 0 {
		c.HeartTimeout = 5 * time.Second
	}
	if c.HeartPorbe <= 0 {
		c.HeartPorbe = 1500 * time.Millisecond
	}
	if c.GroupNum == 0 {
		c.GroupNum = 1
	}
	if c.SocketRBuf == 0 {
		c.SocketRBuf = 1024
	}
	if c.SocketRBuf > 65535 {
		c.SocketRBuf = 65535
	}
	if c.SocketWBuf == 0 {
		c.SocketWBuf = 1024
	}
	if c.SocketWBuf > 65535 {
		c.SocketWBuf = 65535
	}
	if c.MaxMsgLen < 1024 {
		c.MaxMsgLen = 65535
	}
	if c.MaxMsgLen > 65535 {
		c.MaxMsgLen = 65535
	}
	if c.MaxBufferedWriteMsgNum == 0 {
		c.MaxBufferedWriteMsgNum = 256
	}
	if c.DiscoverInterval < time.Second {
		c.DiscoverInterval = time.Second
	}
}

type DiscoveryClient struct {
	selfappname string
	appname     string
	c           *ClientConfig
	instance    *stream.Instance
	selfreginfo *msg.RegInfo

	lker    *sync.RWMutex
	servers map[string]*servernode //key server addr

	manually     chan struct{}
	manualNotice map[chan struct{}]struct{}
	mlker        *sync.Mutex

	rpcnotices map[string]map[chan struct{}]struct{} //key appname
	webnotices map[string]map[chan struct{}]struct{} //key appname
	nlker      *sync.RWMutex
}

var instance *DiscoveryClient

type servernode struct {
	addr        string
	lker        *sync.Mutex
	peer        *stream.Peer
	sid         int64
	allapps     map[string]map[string]*msg.RegInfo //first key:appname,second key:appuniquename
	status      int                                //0-closing,1-start,2-verified,3-connected,4-registered
	selfreginfo *msg.RegInfo
}

func NewDiscoveryClient(c *ClientConfig, selfgroup, selfname, group, name string) error {
	if e := common.NameCheck(selfname, false, true, false, true); e != nil {
		return e
	}
	if e := common.NameCheck(name, false, true, false, true); e != nil {
		return e
	}
	if e := common.NameCheck(selfgroup, false, true, false, true); e != nil {
		return e
	}
	if e := common.NameCheck(group, false, true, false, true); e != nil {
		return e
	}
	appname := group + "." + name
	if e := common.NameCheck(appname, true, true, false, true); e != nil {
		return e
	}
	selfappname := selfgroup + "." + selfname
	if e := common.NameCheck(selfappname, true, true, false, true); e != nil {
		return e
	}
	if c == nil {
		return errors.New("[Discovery.client] missing config")
	}
	if c.DiscoverFunction == nil {
		return errors.New("[Discovery.client] missing discover in config")
	}
	c.validate()
	client := &DiscoveryClient{
		selfappname: selfappname,
		appname:     appname,
		c:           c,

		lker:    &sync.RWMutex{},
		servers: make(map[string]*servernode, 5),

		manually:     make(chan struct{}, 1),
		manualNotice: make(map[chan struct{}]struct{}, 5),
		mlker:        &sync.Mutex{},

		webnotices: make(map[string]map[chan struct{}]struct{}, 5),
		rpcnotices: make(map[string]map[chan struct{}]struct{}, 5),
		nlker:      &sync.RWMutex{},
	}
	if !atomic.CompareAndSwapPointer((*unsafe.Pointer)(unsafe.Pointer(&instance)), nil, unsafe.Pointer(client)) {
		return nil
	}
	tcpc := &stream.InstanceConfig{
		HeartbeatTimeout:       c.HeartTimeout,
		HeartprobeInterval:     c.HeartPorbe,
		MaxBufferedWriteMsgNum: c.MaxBufferedWriteMsgNum,
		GroupNum:               c.GroupNum,
		TcpC: &stream.TcpConfig{
			ConnectTimeout: c.ConnTimeout,
			SocketRBufLen:  c.SocketRBuf,
			SocketWBufLen:  c.SocketWBuf,
			MaxMsgLen:      c.MaxMsgLen,
		},
	}
	//tcp instance
	tcpc.Verifyfunc = client.verifyfunc
	tcpc.Onlinefunc = client.onlinefunc
	tcpc.Userdatafunc = client.userfunc
	tcpc.Offlinefunc = client.offlinefunc
	client.instance, _ = stream.NewInstance(tcpc, selfgroup, selfname)
	client.manually <- struct{}{}
	manualNotice := make(chan struct{}, 1)
	client.manualNotice[manualNotice] = struct{}{}
	go defaultDiscover(group, name, client)
	<-manualNotice
	return nil
}

//server addr format: servername:ip:port
func (c *DiscoveryClient) update(all map[string]struct{}) {
	c.lker.Lock()
	defer c.lker.Unlock()
	//delete offline server
	for _, exist := range c.servers {
		exist.lker.Lock()
		if _, ok := all[exist.addr]; !ok {
			exist.status = 0
			delete(c.servers, exist.addr)
			if exist.peer != nil {
				exist.peer.Close(exist.sid)
			}
		}
		exist.lker.Unlock()
	}
	//online new server
	for addr := range all {
		if _, ok := c.servers[addr]; !ok {
			c.servers[addr] = &servernode{
				addr:        addr,
				lker:        &sync.Mutex{},
				peer:        nil,
				sid:         0,
				allapps:     make(map[string]map[string]*msg.RegInfo, 5),
				status:      1,
				selfreginfo: c.selfreginfo,
			}
			go c.start(addr)
		}
	}
}
func (c *DiscoveryClient) start(addr string) {
	tempverifydata := c.c.VerifyData + "|" + c.appname
	if r := c.instance.StartTcpClient(addr, common.Str2byte(tempverifydata)); r == "" {
		c.lker.RLock()
		server, ok := c.servers[addr]
		if !ok {
			//server removed
			c.lker.RUnlock()
			return
		}
		server.lker.Lock()
		c.lker.RUnlock()
		server.status = 1
		server.lker.Unlock()
		manualNotice := make(chan struct{}, 1)
		c.mlker.Lock()
		c.manualNotice[manualNotice] = struct{}{}
		if len(c.manualNotice) == 1 {
			c.manually <- struct{}{}
		}
		c.mlker.Unlock()
		<-manualNotice
		if server.status == 1 {
			go c.start(addr)
		}
	}
}
func RegisterSelf(reginfo *msg.RegInfo) error {
	if instance == nil {
		return errors.New("[Discovery.client] not inited")
	}
	if reginfo == nil || (reginfo.RpcPort == 0 && reginfo.WebPort == 0) {
		return errors.New("[Discovery.client] register message empty")
	}
	if reginfo.RpcPort > 65535 || reginfo.RpcPort <= 0 {
		return errors.New("[Discovery.client] reginfo's RpcPort out of range")
	}
	if reginfo.WebPort > 65535 || reginfo.WebPort <= 0 {
		return errors.New("[Discovery.client] reginfo's WebPort out of range")
	}
	if reginfo.RpcPort == reginfo.WebPort {
		return errors.New("[Discovery.client] reginfo's RpcPort and WebPort conflict")
	}
	instance.lker.Lock()
	defer instance.lker.Unlock()
	if instance.selfreginfo != nil {
		if instance.selfreginfo.WebPort == reginfo.WebPort &&
			instance.selfreginfo.RpcPort == reginfo.RpcPort {
			return nil
		}
		return errors.New("[Discovery.client] already registered")
	}
	instance.selfreginfo = reginfo
	for addr, server := range instance.servers {
		server.lker.Lock()
		server.selfreginfo = reginfo
		if server.status == 3 {
			server.status = 4
			if reginfo.RpcPort != 0 && reginfo.WebPort != 0 {
				log.Info(nil, "[Discovery.client.RegisterSelf] reg to discovery server:", addr, "with rpc on port:", reginfo.RpcPort, "web on port:", reginfo.WebPort)
			} else if reginfo.RpcPort != 0 {
				log.Info(nil, "[Discovery.client.RegisterSelf] reg to discovery server:", addr, "with rpc on port:", reginfo.RpcPort)
			} else {
				log.Info(nil, "[Discovery.client.RegisterSelf] reg to discovery server:", addr, "with web on port:", reginfo.WebPort)
			}
			onlinemsg, _ := proto.Marshal(&msg.Msg{
				MsgType: msg.MsgType_Reg,
				RegMsg: &msg.RegMsg{
					RegInfo: reginfo,
				},
			})
			server.peer.SendMessage(onlinemsg, server.sid, true)
		}
		server.lker.Unlock()
	}
	return nil
}
func UnRegisterSelf() error {
	if instance == nil {
		return errors.New("[Discovery.client] not inited")
	}
	instance.lker.RLock()
	defer instance.lker.RUnlock()
	instance.selfreginfo = nil
	if len(instance.servers) > 0 {
		offlinemsg, _ := proto.Marshal(&msg.Msg{
			MsgType: msg.MsgType_UnReg,
		})
		for addr, server := range instance.servers {
			server.lker.Lock()
			server.status = 3
			server.selfreginfo = nil
			server.peer.SendMessage(offlinemsg, server.sid, true)
			log.Info(nil, "[Discovery.client] unreg from discovery server:", addr)
			server.lker.Unlock()
		}
	}
	return nil
}

func NoticeWebChanges(appname string) (<-chan struct{}, error) {
	if instance == nil {
		return nil, errors.New("[Discovery.client] not inited")
	}
	watchmsg, _ := proto.Marshal(&msg.Msg{
		MsgType: msg.MsgType_Watch,
		WatchMsg: &msg.WatchMsg{
			AppName: appname,
		},
	})
	instance.lker.RLock()
	for _, server := range instance.servers {
		server.lker.Lock()
		if server.status >= 3 {
			server.peer.SendMessage(watchmsg, server.sid, true)
		}
		server.lker.Unlock()
	}
	instance.lker.RUnlock()
	instance.nlker.Lock()
	defer instance.nlker.Unlock()
	if _, ok := instance.webnotices[appname]; !ok {
		instance.webnotices[appname] = make(map[chan struct{}]struct{}, 10)
	}
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	instance.webnotices[appname][ch] = struct{}{}
	return ch, nil
}

func NoticeRpcChanges(appname string) (<-chan struct{}, error) {
	if instance == nil {
		return nil, errors.New("[Discovery.client] not inited")
	}
	watchmsg, _ := proto.Marshal(&msg.Msg{
		MsgType: msg.MsgType_Watch,
		WatchMsg: &msg.WatchMsg{
			AppName: appname,
		},
	})
	instance.lker.RLock()
	for _, server := range instance.servers {
		server.lker.Lock()
		if server.status >= 3 {
			server.peer.SendMessage(watchmsg, server.sid, true)
		}
		server.lker.Unlock()
	}
	instance.lker.RUnlock()
	instance.nlker.Lock()
	defer instance.nlker.Unlock()
	if _, ok := instance.rpcnotices[appname]; !ok {
		instance.rpcnotices[appname] = make(map[chan struct{}]struct{}, 10)
	}
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	instance.rpcnotices[appname][ch] = struct{}{}
	return ch, nil
}

//key is app addr
func GetRpcInfos(appname string) map[string]*AppRegisterData {
	if instance == nil {
		return nil
	}
	return getinfos(appname, 1)
}

//key is app addr
func GetWebInfos(appname string) map[string]*AppRegisterData {
	if instance == nil {
		return nil
	}
	return getinfos(appname, 2)
}

type AppRegisterData struct {
	DiscoveryServerAddrs map[string]struct{}
	Addition             []byte
}

func getinfos(appname string, t int) map[string]*AppRegisterData {
	result := make(map[string]*AppRegisterData)
	instance.lker.RLock()
	defer instance.lker.RUnlock()
	for _, server := range instance.servers {
		server.lker.Lock()
		if apps, ok := server.allapps[appname]; ok {
			for _, app := range apps {
				var addr string
				switch t {
				case 1:
					if app.RpcPort == 0 {
						continue
					}
					addr = app.RpcIp + ":" + strconv.FormatInt(app.RpcPort, 10)
				case 2:
					if app.WebPort == 0 {
						continue
					}
					addr = app.WebIp + ":" + strconv.FormatInt(app.WebPort, 10)
				}
				if _, ok := result[addr]; !ok {
					result[addr] = &AppRegisterData{
						DiscoveryServerAddrs: make(map[string]struct{}),
					}
				}
				result[addr].DiscoveryServerAddrs[server.addr] = struct{}{}
				result[addr].Addition = app.Addition
			}
		}
		server.lker.Unlock()
	}
	return result
}

func (c *DiscoveryClient) verifyfunc(ctx context.Context, serveruniquename string, peerVerifyData []byte) ([]byte, bool) {
	if common.Byte2str(peerVerifyData) != c.c.VerifyData {
		return nil, false
	}
	c.lker.RLock()
	addr := serveruniquename[strings.Index(serveruniquename, ":")+1:]
	exist, ok := c.servers[addr]
	if !ok {
		//discovery server removed
		c.lker.RUnlock()
		return nil, false
	}
	exist.lker.Lock()
	c.lker.RUnlock()
	exist.status = 2
	exist.lker.Unlock()
	return nil, true
}
func (c *DiscoveryClient) onlinefunc(p *stream.Peer, serveruniquename string, sid int64) bool {
	c.lker.RLock()
	addr := serveruniquename[strings.Index(serveruniquename, ":")+1:]
	exist, ok := c.servers[addr]
	if !ok {
		//discovery server removed
		c.lker.RUnlock()
		return false
	}
	exist.lker.Lock()
	defer exist.lker.Unlock()
	c.lker.RUnlock()
	log.Info(nil, "[Discovery.client.onlinefunc] discovery server:", addr, "online")
	exist.peer = p
	exist.sid = sid
	exist.status = 3
	p.SetData(unsafe.Pointer(exist))
	if exist.selfreginfo != nil {
		exist.status = 4
		if exist.selfreginfo.RpcPort != 0 && exist.selfreginfo.WebPort != 0 {
			log.Info(nil, "[Discovery.client.onlinefunc] reg to discovery server:", addr, "with rpc on port:", exist.selfreginfo.RpcPort, "web on port:", exist.selfreginfo.WebPort)
		} else if exist.selfreginfo.RpcPort != 0 {
			log.Info(nil, "[Discovery.client.onlinefunc] reg to discovery server:", addr, "with rpc on port:", exist.selfreginfo.RpcPort)
		} else {
			log.Info(nil, "[Discovery.client.onlinefunc] reg to discovery server:", addr, "with web on port:", exist.selfreginfo.WebPort)
		}
		onlinemsg, _ := proto.Marshal(&msg.Msg{
			MsgType: msg.MsgType_Reg,
			RegMsg: &msg.RegMsg{
				RegInfo: exist.selfreginfo,
			},
		})
		exist.peer.SendMessage(onlinemsg, exist.sid, true)
	}
	c.nlker.RLock()
	result := make(map[string]struct{}, 5)
	for k := range c.rpcnotices {
		result[k] = struct{}{}
	}
	for k := range c.webnotices {
		result[k] = struct{}{}
	}
	c.nlker.RUnlock()
	for k := range result {
		watchmsg, _ := proto.Marshal(&msg.Msg{
			MsgType: msg.MsgType_Watch,
			WatchMsg: &msg.WatchMsg{
				AppName: k,
			},
		})
		exist.peer.SendMessage(watchmsg, exist.sid, true)
	}
	return true
}
func (c *DiscoveryClient) userfunc(p *stream.Peer, serveruniquename string, origindata []byte, sid int64) {
	if len(origindata) == 0 {
		return
	}
	server := (*servernode)(p.GetData())
	data := make([]byte, len(origindata))
	copy(data, origindata)
	m := &msg.Msg{}
	if e := proto.Unmarshal(data, m); e != nil {
		log.Error(nil, "[Discovery.client.userfunc] message from:", server.addr, "format error:", e)
		p.Close(sid)
		return
	}
	server.lker.Lock()
	defer server.lker.Unlock()
	switch m.MsgType {
	case msg.MsgType_Reg:
		reg := m.GetRegMsg()
		if reg == nil || reg.AppUniqueName == "" || reg.RegInfo == nil || (reg.RegInfo.WebPort == 0 && reg.RegInfo.RpcPort == 0) {
			log.Error(nil, "[Discovery.client.userfunc] empty reg msg from:", server.addr)
			p.Close(sid)
			return
		}
		appname := reg.AppUniqueName[:strings.Index(reg.AppUniqueName, ":")]
		if apps, ok := server.allapps[appname]; ok {
			if _, ok := apps[reg.AppUniqueName]; ok {
				return
			}
		}
		if _, ok := server.allapps[appname]; !ok {
			server.allapps[appname] = make(map[string]*msg.RegInfo, 5)
		}
		server.allapps[appname][reg.AppUniqueName] = reg.RegInfo
		c.nlker.RLock()
		c.notice(appname)
		c.nlker.RUnlock()
	case msg.MsgType_UnReg:
		unreg := m.GetUnregMsg()
		if unreg.AppUniqueName == "" {
			log.Error(nil, "[Discovery.client.userfunc] empty unreg msg from:", server.addr)
			p.Close(sid)
			return
		}
		appname := unreg.AppUniqueName[:strings.Index(unreg.AppUniqueName, ":")]
		delete(server.allapps[appname], unreg.AppUniqueName)
		if len(server.allapps[appname]) == 0 {
			delete(server.allapps, appname)
		}
		c.nlker.RLock()
		c.notice(appname)
		c.nlker.RUnlock()
	case msg.MsgType_Push:
		push := m.GetPushMsg()
		if push == nil || push.AppName == "" {
			log.Error(nil, "[Discovery.client.userfunc] empty push msg from:", server.addr)
			p.Close(sid)
			return
		}
		if _, ok := server.allapps[push.AppName]; !ok {
			server.allapps[push.AppName] = make(map[string]*msg.RegInfo, 5)
		}
		//offline
		for appuniquename := range server.allapps[push.AppName] {
			if _, ok := push.Apps[appuniquename]; !ok {
				delete(server.allapps, appuniquename)
			}
		}
		//online
		for appuniquename, reginfo := range push.Apps {
			server.allapps[push.AppName][appuniquename] = reginfo
		}
		c.nlker.RLock()
		c.notice(push.AppName)
		c.nlker.RUnlock()
	default:
		log.Error(nil, "[Discovery.client.userfunc] unknown message type from:", server.addr)
		p.Close(sid)
	}
}
func (c *DiscoveryClient) offlinefunc(p *stream.Peer, serveruniquename string) {
	server := (*servernode)(p.GetData())
	log.Info(nil, "[Discovery.client.offlinefunc] discovery server:", server.addr, "offline")
	server.lker.Lock()
	defer server.lker.Unlock()
	server.peer = nil
	server.sid = 0
	//notice
	c.nlker.RLock()
	for appname := range server.allapps {
		c.notice(appname)
	}
	c.nlker.RUnlock()
	server.allapps = make(map[string]map[string]*msg.RegInfo, 5)
	if server.status != 0 {
		server.status = 1
		go c.start(server.addr)
	}
}
func (c *DiscoveryClient) notice(appname string) {
	if notices, ok := c.rpcnotices[appname]; ok {
		for n := range notices {
			select {
			case n <- struct{}{}:
			default:
			}
		}
	}
	if notices, ok := c.webnotices[appname]; ok {
		for n := range notices {
			select {
			case n <- struct{}{}:
			default:
			}
		}
	}
}
