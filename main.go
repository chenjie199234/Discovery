package main

import (
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chenjie199234/Corelib/discovery"
	"github.com/chenjie199234/Corelib/log"
	"github.com/chenjie199234/Corelib/web"
)

func main() {
	defer log.Close()
	var verifydatas []string
	if str, ok := os.LookupEnv("SERVER_VERIFY_DATA"); ok && str != "<SERVER_VERIFY_DATA>" {
		verifydatas = make([]string, 0)
		if e := json.Unmarshal([]byte(str), &verifydatas); e != nil {
			log.Error("[main] SERVER_VERIFY_DATA must be json string array like:[\"abc\",\"123\"]")
			return
		}
	} else {
		log.Warning("[main] missing SERVER_VERIFY_DATA")
	}
	discoveryserver, e := discovery.NewDiscoveryServer(nil, "default", "discovery", verifydatas)
	if e != nil {
		log.Error("[main] new discovery server error:", e)
		return
	}
	webserver, e := web.NewWebServer(&web.Config{
		Timeout:            time.Millisecond * 500,
		StaticFileRootPath: "./src",
		MaxHeader:          1024,
		ReadBuffer:         1024,
		WriteBuffer:        1024,
		Cors: &web.CorsConfig{
			AllowedOrigin:    []string{"*"},
			AllowedHeader:    []string{"*"},
			ExposeHeader:     nil,
			AllowCredentials: false,
			MaxAge:           time.Hour * 24,
		},
	}, "default", "discovery")
	if e != nil {
		log.Error("[main] new web server error:", e)
		return
	}
	webserver.Get("/infos", time.Millisecond*500, func(ctx *web.Context) {
		result := discoveryserver.GetAppInfos()
		d, _ := json.Marshal(result)
		ctx.Write(200, d)
	})
	webserver.Get("/info/:group/:name", time.Millisecond*500, func(ctx *web.Context) {
		group := ctx.GetParam("group")
		name := ctx.GetParam("name")
		if group != "" && name != "" {
			result := discoveryserver.GetAppInfo(group + "." + name)
			d, _ := json.Marshal(result)
			ctx.Write(200, d)
		} else {
			ctx.WriteString(404, "bad request")
		}
	})
	ch := make(chan os.Signal, 1)
	go func() {
		if e := discoveryserver.StartDiscoveryServer(":10000"); e != nil {
			log.Error("[main] start discovery server error:", e)
		}
		select {
		case ch <- syscall.SIGTERM:
		default:
		}
	}()
	go func() {
		if e := webserver.StartWebServer(":8000", "", ""); e != nil {
			log.Error("[main] start web server error:", e)
		}
		select {
		case ch <- syscall.SIGTERM:
		default:
		}
	}()
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-ch
	discoveryserver.StopDiscoveryServer()
}
