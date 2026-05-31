package main

import (
	"log"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ipipdotnet/ipdb-go"
	"github.com/json-iterator/go/extra"
)

var dbip *ipdb.City

func main() {
	extra.RegisterFuzzyDecoders()

	conf.init()

	initTokenKey()

	var err error

	dbip, err = ipdb.NewCity(conf.System.IPfile)
	if err != nil {
		log.Fatal(err)
	}

	setQTH("MEETLY-201", "会议模式", qth{"会议模式", "MEETLY-201", time.Now(), "多人同时讲话"})

	db = getDB()

	updatedb()

	chatgptInit()

	initAllUserList()

	initPublicGroup()

	initAllDevList()

	initHomepageTables()

	go jsonhttp.init()
	go callWSHub.run()

	logbuffer = make(chan *deviceInfo, 1000)
	go saveLog()

	go cronGetWxToken()

	go checkdeviceOnline()

	go NewAPRS().OnLoad()

	go startPlatformServerSync()

	// 优雅关闭
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		sig := <-sigCh
		log.Printf("received signal %v, shutting down...", sig)
		close(logbuffer)
		if globelconn != nil {
			globelconn.Close()
		}
		os.Exit(0)
	}()

	udpServer()
}
