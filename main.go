package main

import (
	"log"
	_ "net/http/pprof"
	"time"

	"github.com/ipipdotnet/ipdb-go"
	"github.com/json-iterator/go/extra"
)

var dbip *ipdb.City

func main() {

	// fmt.Println("heck database support ipv4:", db.IsIPv4())     // check database support ip type
	// fmt.Println("check database support ip type:", db.IsIPv6()) // check database support ip type
	// fmt.Println("database build time:", db.BuildTime())         // database build time
	// fmt.Println("database support language:", db.Languages())   // database support language
	// fmt.Println("database support fields:", db.Fields())        // database support fields

	extra.RegisterFuzzyDecoders()

	conf.init()

	initTokenKey()

	var err error

	dbip, err = ipdb.NewCity(conf.System.IPfile)
	if err != nil {
		if !Exist(conf.System.IPfile) {
			// IP库缺失时降级运行（QTH定位不可用），不阻断启动
			log.Printf("IP库文件不存在: %s，QTH定位功能不可用（可挂载 udphub.ipdb 并用 NRL_IPFILE 指定路径）", conf.System.IPfile)
		} else {
			log.Fatal(err)
		}
	}

	setQTH("MEETLY-201", "会议模式", qth{"会议模式", "MEETLY-201", time.Now(), "多人同时讲话"})

	db = getDB()

	execDDL()

	updatedb()
	initSiteSettingTable()
	initHomepageTables()

	ensureBootstrap()

	chatgptInit()

	initAllUserList()

	initPublicGroup()

	initAllDevList()

	go jsonhttp.init()
	go callWSHub.run()

	logbuffer = make(chan *deviceInfo, 1000)
	go saveLog()

	go cronGetWxToken()

	go checkdeviceOnline()

	go NewAPRS().OnLoad()

	//go findNRL()

	go startPlatformServerSync()

	udpServer()

}
