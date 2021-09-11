package server

import (
	"context"
	"errors"
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

const (
	s_CLOSED = iota
	s_CONNECTED
	s_REGISTERED
)

type ServerConfig struct {
	//when server close,server will wait this time before close,every request will refresh the time
	//min is 1 second
	HeartTimeout           time.Duration
	HeartPorbe             time.Duration
	GroupNum               uint32
	SocketRBuf             uint32
	SocketWBuf             uint32
	MaxMsgLen              uint32
	MaxBufferedWriteMsgNum uint32
	VerifyDatas            []string
}

func (c *ServerConfig) validate() {
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
}

type DiscoveryServer struct {
	c         *ServerConfig
	lker      *sync.RWMutex
	apps      map[string]*app //key appname
	instance  *stream.Instance
	closewait *sync.WaitGroup
	closing   int32
}

//appuniquename = appname:ip:port
type app struct {
	nodes    map[string]*appnode //key appuniquename
	watchers map[string]*appnode //key appuniquename
}

//appuniquename = appname:ip:port
type appnode struct {
	lker          *sync.RWMutex
	appuniquename string
	peer          *stream.Peer
	sid           int64
	status        int
	reginfo       *msg.RegInfo
	watched       map[string]struct{} //key appname
	bewatched     map[string]*appnode //key appuniquename
}

//old: true means oldverifydata is useful,false means oldverifydata is useless
func NewDiscoveryServer(c *ServerConfig, group, name string) (*DiscoveryServer, error) {
	if e := common.NameCheck(name, false, true, false, true); e != nil {
		return nil, e
	}
	if e := common.NameCheck(group, false, true, false, true); e != nil {
		return nil, e
	}
	if e := common.NameCheck(group+"."+name, true, true, false, true); e != nil {
		return nil, e
	}
	if c == nil {
		c = &ServerConfig{}
	}
	c.validate()
	instance := &DiscoveryServer{
		c:         c,
		lker:      &sync.RWMutex{},
		apps:      make(map[string]*app, 5),
		closewait: &sync.WaitGroup{},
	}
	instance.closewait.Add(1)
	tcpc := &stream.InstanceConfig{
		HeartbeatTimeout:       c.HeartTimeout,
		HeartprobeInterval:     c.HeartPorbe,
		MaxBufferedWriteMsgNum: c.MaxBufferedWriteMsgNum,
		GroupNum:               c.GroupNum,
		TcpC: &stream.TcpConfig{
			SocketRBufLen: c.SocketRBuf,
			SocketWBufLen: c.SocketWBuf,
			MaxMsgLen:     c.MaxMsgLen,
		},
	}
	//tcp instance
	tcpc.Verifyfunc = instance.verifyfunc
	tcpc.Onlinefunc = instance.onlinefunc
	tcpc.Userdatafunc = instance.userfunc
	tcpc.Offlinefunc = instance.offlinefunc
	instance.instance, _ = stream.NewInstance(tcpc, group, name)
	return instance, nil
}

var ErrServerClosed = errors.New("[discovery.server] closed")
var ErrAlreadyStarted = errors.New("[discovery.server] already started")

func (s *DiscoveryServer) StartDiscoveryServer(listenaddr string) error {
	e := s.instance.StartTcpServer(listenaddr, nil)
	if e == stream.ErrServerClosed {
		return ErrServerClosed
	} else if e == stream.ErrAlreadyStarted {
		return ErrAlreadyStarted
	}
	return e
}
func (s *DiscoveryServer) StopDiscoveryServer() {
	if atomic.SwapInt32(&s.closing, 1) == 0 {
		s.instance.Stop()
		s.closewait.Done()
	}
	s.closewait.Wait()
}

//one app's info
type Info struct {
	Apps     map[string]string //key appuniquename,value registered status
	Watchers []string          //value appuniquename
}

func (s *DiscoveryServer) GetAppInfos() map[string]*Info {
	result := make(map[string]*Info)
	s.lker.RLock()
	defer s.lker.RUnlock()
	for appname, app := range s.apps {
		temp := &Info{
			Apps:     make(map[string]string, len(app.nodes)),
			Watchers: make([]string, 0, len(app.watchers)),
		}
		for appuniquename, node := range app.nodes {
			switch node.status {
			case s_CONNECTED:
				temp.Apps[appuniquename] = "connected"
			case s_REGISTERED:
				temp.Apps[appuniquename] = "registered"
			case s_CLOSED:
				temp.Apps[appuniquename] = "closed"
			}
		}
		for watcheruniquename := range app.watchers {
			temp.Watchers = append(temp.Watchers, watcheruniquename)
		}
		result[appname] = temp
	}
	return result
}
func (s *DiscoveryServer) GetAppInfo(appname string) map[string]*Info {
	result := make(map[string]*Info)
	s.lker.RLock()
	defer s.lker.RUnlock()
	if app, ok := s.apps[appname]; ok {
		temp := &Info{
			Apps:     make(map[string]string, len(app.nodes)),
			Watchers: make([]string, 0, len(app.watchers)),
		}
		for appuniquename, node := range app.nodes {
			switch node.status {
			case s_CONNECTED:
				temp.Apps[appuniquename] = "connected"
			case s_REGISTERED:
				temp.Apps[appuniquename] = "registered"
			case s_CLOSED:
				temp.Apps[appuniquename] = "closed"
			}
		}
		for watcheruniquename := range app.watchers {
			temp.Watchers = append(temp.Watchers, watcheruniquename)
		}
		result[appname] = temp
	}
	return result
}

//appuniquename = appname:ip:port
func (s *DiscoveryServer) verifyfunc(ctx context.Context, appuniquename string, peerVerifyData []byte) ([]byte, bool) {
	if s.closing == 1 {
		return nil, false
	}
	temp := common.Byte2str(peerVerifyData)
	index := strings.LastIndex(temp, "|")
	if index == -1 {
		return nil, false
	}
	targetname := temp[index+1:]
	vdata := temp[:index]
	if targetname != s.instance.GetSelfName() {
		return nil, false
	}
	if len(s.c.VerifyDatas) == 0 {
		dup := make([]byte, len(vdata))
		copy(dup, vdata)
		return dup, true
	}
	for _, verifydata := range s.c.VerifyDatas {
		if verifydata == vdata {
			return common.Str2byte(verifydata), true
		}
	}
	return nil, false
}

//appuniquename = appname:ip:port
func (s *DiscoveryServer) onlinefunc(p *stream.Peer, appuniquename string, sid int64) bool {
	if s.closing == 1 {
		return false
	}
	s.lker.Lock()
	defer s.lker.Unlock()
	appname := appuniquename[:strings.Index(appuniquename, ":")]
	if g, ok := s.apps[appname]; ok {
		if _, ok := g.nodes[appuniquename]; ok {
			p.Close(sid)
			return false
		}
	} else {
		s.apps[appname] = &app{
			nodes:    make(map[string]*appnode, 5),
			watchers: make(map[string]*appnode, 5),
		}
	}
	node := &appnode{
		lker:          new(sync.RWMutex),
		appuniquename: appuniquename,
		peer:          p,
		sid:           sid,
		status:        s_CONNECTED,
		watched:       make(map[string]struct{}, 5),
		bewatched:     make(map[string]*appnode, 5),
	}
	//copy bewarched
	for k, v := range s.apps[appname].watchers {
		node.bewatched[k] = v
	}
	p.SetData(unsafe.Pointer(node))
	s.apps[appname].nodes[appuniquename] = node
	log.Info(nil, "[Discovery.server.onlinefunc] app:", appuniquename, "online")
	return true
}

//appuniquename = appname:ip:port
func (s *DiscoveryServer) userfunc(p *stream.Peer, appuniquename string, origindata []byte, sid int64) {
	if len(origindata) == 0 {
		return
	}
	data := make([]byte, len(origindata))
	copy(data, origindata)
	m := &msg.Msg{}
	if e := proto.Unmarshal(data, m); e != nil {
		log.Error(nil, "[Discovery.server.userfunc] message from:", appuniquename, "format error:", e)
		p.Close(sid)
		return
	}
	switch m.MsgType {
	case msg.MsgType_Reg:
		reg := m.GetRegMsg()
		if reg == nil || reg.RegInfo == nil || (reg.RegInfo.WebPort == 0 && reg.RegInfo.RpcPort == 0) {
			//register with empty data
			log.Error(nil, "[Discovery.server.userfunc] empty reginfo from:", appuniquename)
			p.Close(sid)
			return
		}
		findex := strings.Index(appuniquename, ":")
		lindex := strings.LastIndex(appuniquename, ":")
		ip := appuniquename[findex+1 : lindex]
		reg.RegInfo.WebIp = ""
		if reg.RegInfo.WebPort != 0 {
			reg.RegInfo.WebIp = ip
		}
		reg.RegInfo.RpcIp = ""
		if reg.RegInfo.RpcPort != 0 {
			reg.RegInfo.RpcIp = ip
		}
		node := (*appnode)(p.GetData())
		node.lker.Lock()
		defer node.lker.Unlock()
		node.reginfo = reg.RegInfo
		node.status = s_REGISTERED
		if len(node.bewatched) > 0 {
			onlinemsg, _ := proto.Marshal(&msg.Msg{
				MsgType: msg.MsgType_Reg,
				RegMsg: &msg.RegMsg{
					AppUniqueName: appuniquename,
					RegInfo:       reg.RegInfo,
				},
			})
			for _, watcherapp := range node.bewatched {
				watcherapp.lker.RLock()
				if watcherapp.status != s_CLOSED {
					watcherapp.peer.SendMessage(onlinemsg, watcherapp.sid, true)
				}
				watcherapp.lker.RUnlock()
			}
		}
		if reg.RegInfo.WebIp != "" && reg.RegInfo.RpcIp != "" {
			log.Info(nil, "[Discovery.server.userfunc] app:", appuniquename, "reg with rpc:", ip, reg.RegInfo.RpcPort, "web:", ip, reg.RegInfo.WebPort)
		} else if reg.RegInfo.WebIp != "" {
			log.Info(nil, "[Discovery.server.userfunc] app:", appuniquename, "reg with web:", ip, reg.RegInfo.WebPort)
		} else {
			log.Info(nil, "[Discovery.server.userfunc] app:", appuniquename, "reg with rpc:", ip, reg.RegInfo.RpcPort)
		}
	case msg.MsgType_Watch:
		watch := m.GetWatchMsg()
		if watch.AppName == "" {
			log.Error(nil, "[Discovery.server.userfunc] app:", appuniquename, "watch empty")
			p.Close(sid)
			return
		}
		if watch.AppName == appuniquename[:strings.Index(appuniquename, ":")] {
			log.Error(nil, "[Discovery.server.userfunc] app:", appuniquename, "watch self")
			p.Close(sid)
			return
		}
		node := (*appnode)(p.GetData())
		s.lker.Lock()
		if _, ok := s.apps[watch.AppName]; !ok {
			s.apps[watch.AppName] = &app{
				nodes:    make(map[string]*appnode, 5),
				watchers: make(map[string]*appnode, 5),
			}
		}
		s.apps[watch.AppName].watchers[appuniquename] = node
		reginfos := make(map[string]*msg.RegInfo, len(s.apps[watch.AppName].nodes))
		for _, node := range s.apps[watch.AppName].nodes {
			node.lker.Lock()
			node.bewatched[appuniquename] = node
			if node.status == s_REGISTERED {
				reginfos[node.appuniquename] = node.reginfo
			}
			node.lker.Unlock()
		}
		node.lker.Lock()
		s.lker.Unlock()
		node.watched[watch.AppName] = struct{}{}
		pushmsg, _ := proto.Marshal(&msg.Msg{
			MsgType: msg.MsgType_Push,
			PushMsg: &msg.PushMsg{
				AppName: watch.AppName,
				Apps:    reginfos,
			},
		})
		node.peer.SendMessage(pushmsg, node.sid, true)
		node.lker.Unlock()
	case msg.MsgType_UnReg:
		node := (*appnode)(p.GetData())
		node.lker.Lock()
		defer node.lker.Unlock()
		if node.status == s_REGISTERED {
			node.status = s_CONNECTED
			if len(node.bewatched) > 0 {
				offlinemsg, _ := proto.Marshal(&msg.Msg{
					MsgType: msg.MsgType_UnReg,
					UnregMsg: &msg.UnregMsg{
						AppUniqueName: appuniquename,
					},
				})
				for _, watcherapp := range node.bewatched {
					watcherapp.lker.RLock()
					if watcherapp.status != s_CLOSED {
						watcherapp.peer.SendMessage(offlinemsg, watcherapp.sid, true)
					}
					watcherapp.lker.RUnlock()
				}
			}
		}
		log.Info(nil, "[Discovery.server.userfunc] app:", appuniquename, "unreg")
	default:
		log.Error(nil, "[Discovery.server.userfunc] unknown message type:", m.MsgType, "from app:", appuniquename)
		p.Close(sid)
	}
}

//appuniquename = appname:ip:port
func (s *DiscoveryServer) offlinefunc(p *stream.Peer, appuniquename string) {
	s.lker.Lock()
	appname := appuniquename[:strings.Index(appuniquename, ":")]
	self := s.apps[appname].nodes[appuniquename]
	//app, _ := s.apps[appname]
	//node, _ := app.nodes[appuniquename]
	delete(s.apps[appname].nodes, appuniquename)
	if len(s.apps[appname].nodes) == 0 && len(s.apps[appname].watchers) == 0 {
		delete(s.apps, appname)
	}
	for v := range self.watched {
		bewatchedapp, ok := s.apps[v]
		if !ok {
			continue
		}
		delete(bewatchedapp.watchers, appuniquename)
		for _, node := range bewatchedapp.nodes {
			node.lker.Lock()
			delete(node.bewatched, appuniquename)
			node.lker.Unlock()
		}
		if len(bewatchedapp.nodes) == 0 && len(bewatchedapp.watchers) == 0 {
			delete(s.apps, v)
		}
	}
	self.lker.Lock()
	s.lker.Unlock()
	if self.status == s_REGISTERED && len(self.bewatched) > 0 {
		offlinemsg, _ := proto.Marshal(&msg.Msg{
			MsgType: msg.MsgType_UnReg,
			UnregMsg: &msg.UnregMsg{
				AppUniqueName: appuniquename,
			},
		})
		for _, watcherapp := range self.bewatched {
			watcherapp.lker.RLock()
			if watcherapp.status != s_CLOSED {
				watcherapp.peer.SendMessage(offlinemsg, watcherapp.sid, true)
			}
			watcherapp.lker.RUnlock()
		}
	}
	self.status = s_CLOSED
	self.lker.Unlock()
	log.Info(nil, "[Discovery.server.offlinefunc] app:", appuniquename, "offline")
}
