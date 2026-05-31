package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	jsoniter "github.com/json-iterator/go"

	"golang.org/x/net/websocket"
)

var jsonextra = jsoniter.ConfigCompatibleWithStandardLibrary

type jsonapi struct {
}

var jsonhttp = &jsonapi{}

func (j *jsonapi) init() {

	//getyouzhantoken()

	j.msghttp()

}

type platforminfo struct {
	Name     string `json:"name"`
	LogoURL  string `json:"logourl"`
	Version  string `json:"version"`
	ICP      string `json:"icp"`
	Mail     string `json:"mail"`
	Callsign string `json:"callsign"`
	Language string `json:"language"`
}

var totalstats = totalStats{}
var totalstatsMu sync.RWMutex

type totalStats struct {
	DevNumber           int `json:"dev_number"`
	OnlineDevNumber     int `json:"online_dev_number"`
	UserNumber          int `json:"user_number"`
	VoiceTime           int `json:"voice_time"`
	Traffic             int `json:"traffic"`
	PacketNumber        int `json:"packet_number"`
	SessionNumber       int `json:"session_number"`
	MsgNumber           int `json:"msg_number"`
	LostPercent         int `json:"lost_percent"`
	PlatformDevOnline   int `json:"platform_dev_online"`
	PlatformDevTotal    int `json:"platform_dev_total"`
	PlatformServerTotal int `json:"platform_server_total"`
	PlatformBoxTotal    int `json:"platform_box_total"`
	PlatformAppTotal    int `json:"platform_app_total"`
	PlatformMPTotal     int `json:"platform_mptotal"`
}

func (j *jsonapi) httpTotalStats(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	totalstatsMu.Lock()
	totalstats.DevNumber = devMapLen()
	totalstats.OnlineDevNumber = currentOnlineDeviceCount()
	totalstats.UserNumber = 1000
	totalstatsMu.Unlock()

	totalstatsMu.RLock()
	rescode, _ := jsonextra.Marshal(totalstats)
	totalstatsMu.RUnlock()

	respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)
	w.Write([]byte(respone))
}

func (j *jsonapi) httpplatforminfo(w http.ResponseWriter, req *http.Request) {

	p := platforminfo{
		Name:     conf.SystemInfo.PlatformName,
		LogoURL:  conf.SystemInfo.LogoURL,
		Language: conf.SystemInfo.Language,
		Version:  "v2.0.0",
		ICP:      conf.Web.ICP,
		Mail:     "caoc@live.com",
		Callsign: "BH4RPN",
	}

	rescode, _ := jsonextra.Marshal(p)

	respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)

	w.Write([]byte(respone))
}

func (j *jsonapi) httpplatformList(w http.ResponseWriter, req *http.Request) {

	rescode, _ := jsonextra.Marshal(conf.PlatformList)

	respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)

	w.Write([]byte(respone))
}

func (j *jsonapi) httpHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","service":"nrllink-udphub"}`))
}

func sethttphead(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Add("Access-Control-Allow-Headers", "x-token")
	w.Header().Set("content-type", "application/json")
}

func addTotalStatsPacket() {
	totalstatsMu.Lock()
	totalstats.PacketNumber++
	totalstatsMu.Unlock()
}

func addTotalStatsTraffic(n int) {
	totalstatsMu.Lock()
	totalstats.Traffic += n
	totalstatsMu.Unlock()
}

func addTotalStatsVoiceTime(n int) {
	totalstatsMu.Lock()
	totalstats.VoiceTime += n
	totalstatsMu.Unlock()
}

func setTotalStatsOnlineDev(n int) {
	totalstatsMu.Lock()
	totalstats.OnlineDevNumber = n
	totalstatsMu.Unlock()
}

func getTotalStatsOnlineDev() int {
	totalstatsMu.RLock()
	defer totalstatsMu.RUnlock()
	return totalstats.OnlineDevNumber
}

func writeJSONResponseItems(w http.ResponseWriter, data interface{}, total int) {
	rescode, err := jsonextra.Marshal(data)
	if err != nil {
		w.Write([]byte(`{"code":20001,"data":{"message":"JSON序列化失败"}}`))
		return
	}
	var respone string
	if total > 0 {
		respone = fmt.Sprintf(`{"code":20000,"data":{"total":%v,"items":%s}}`, total, rescode)
	} else {
		respone = fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)
	}
	w.Write([]byte(respone))
}

func writeJSONResponseItem(w http.ResponseWriter, data interface{}) {
	rescode, err := jsonextra.Marshal(data)
	if err != nil {
		w.Write([]byte(`{"code":20001,"data":{"message":"JSON序列化失败"}}`))
		return
	}
	respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)
	w.Write([]byte(respone))
}

func writeJSONResponseMessage(w http.ResponseWriter, message string, code int) {
	if code == 0 {
		code = 20000
	}
	respone := fmt.Sprintf(`{"code":%d,"data":{"message":"%s"}}`, code, message)
	w.Write([]byte(respone))
}

func writeJSONResponseError(w http.ResponseWriter, message string) {
	writeJSONResponseMessage(w, message, 20001)
}

func writeJSONResponseOK(w http.ResponseWriter) {
	writeJSONResponseMessage(w, "操作成功", 20000)
}

func writeJSONResponseParamError(w http.ResponseWriter) {
	writeJSONResponseError(w, "参数错误")
}

func writeJSONResponseRightError(w http.ResponseWriter) {
	writeJSONResponseError(w, "权限不足")
}

func writeJSONResponseOpError(w http.ResponseWriter) {
	writeJSONResponseError(w, "操作失败")
}

func allowWebsocketHandshake(config *websocket.Config, req *http.Request) error {
	origin := req.Header.Get("Origin")
	if origin != "" {
		return nil
	}
	return nil
}

func (j *jsonapi) msghttp() {

	router := mux.NewRouter()

	router.HandleFunc("/api/msg/weixin", j.httpWXMsg)               //微信公众号接口
	router.HandleFunc("/weixinreturn/msgstatus", j.httpWXMsgReturn) //微信回调
	router.HandleFunc("/weixin/phonecode", j.httpPhoneCode)         //crm自己处理操作员的过来的绑定请求
	router.HandleFunc("/weixin/mpphonecode", j.httpMPPhoneCode)     //crm自己处理微信小程序后端的过来的绑定请求
	router.HandleFunc("/weixin/wxmsg", j.httpgetWeiXinMsg)          //查询微信用户发过来的消息
	router.HandleFunc("/api/getwxmsg", j.httpGetWeiXinMsgContent)

	//小程序登录
	router.HandleFunc("/api/weixin/wxlogin/teacher", j.httpMPuserLogin)

	http.HandleFunc("/platform/info", j.httpplatforminfo)
	http.HandleFunc("/platform/list", j.httpplatformList)
	http.HandleFunc("/platform/totalstats", j.httpTotalStats)

	http.HandleFunc("/device/list", j.httpDeviceList)
	http.HandleFunc("/device/db/list", j.httpDevicesList)

	http.HandleFunc("/device/get", j.httpDevice)
	http.HandleFunc("/device/qthmap", j.httpDeviceQTHs)
	http.HandleFunc("/device/qth", j.httpDeviceQTHs)
	http.HandleFunc("/device/qths", j.httpDeviceQTHs2)
	http.HandleFunc("/device/qth2", j.httpDeviceQTH)

	http.HandleFunc("/device/mydevlist", j.httpMyDeviceList)
	// http.HandleFunc("/device/binddevice", j.httpBindDevice)
	http.HandleFunc("/device/update", j.httpUpdateDevice)
	http.HandleFunc("/device/delete", j.httpDeleteDevice)

	http.HandleFunc("/device/changegroupnrl", j.httpChangeDeviceGroupNRL)

	http.HandleFunc("/device/query", j.httpQueryDeviceParm)
	http.HandleFunc("/device/change", j.httpChangeDeviceParm)
	http.HandleFunc("/device/change1w", j.httpChange1W)
	http.HandleFunc("/device/change2w", j.httpChange2W)
	http.HandleFunc("/device/at", j.httpDeviceAT)

	http.HandleFunc("/group/get", j.httpGetGroup)

	http.HandleFunc("/group/list/mini", j.httpGetGroupList)

	http.HandleFunc("/group/list", j.httpPublicGroupList)
	http.HandleFunc("/group/device/list", j.httpGroupDeviceList)
	http.HandleFunc("/group/create", j.httpAddGroup)
	http.HandleFunc("/group/update", j.httpUpdateGroup)
	http.HandleFunc("/group/delete", j.httpDeleteGroup)

	http.HandleFunc("/group/listnrl", j.httpAllGroupListNRL)

	http.HandleFunc("/room/list", j.httpRoomList)

	http.HandleFunc("/server/list", j.httpServersList)
	http.HandleFunc("/server/create", j.httpAddServer)
	http.HandleFunc("/server/update", j.httpUpdateServer)
	http.HandleFunc("/server/delete", j.httpDeleteServer)

	http.HandleFunc("/relay/list", j.httpGetRelayList)
	http.HandleFunc("/relay/create", j.httpAddrelay)
	http.HandleFunc("/relay/update", j.httpUpdaterelay)
	http.HandleFunc("/relay/delete", j.httpDeleterelay)

	// http.HandleFunc("/device/create", j.httpAddDevice)
	// http.HandleFunc("/device/update", j.httpUpdateDevice)
	// http.HandleFunc("/device/delete", j.httpDeleteDevice)
	http.HandleFunc("/user/reg/create", j.httpRegister)
	http.HandleFunc("/user/reg/add", j.httpAddReg)
	http.HandleFunc("/user/reg/update", j.httpUpdateReg)
	http.HandleFunc("/user/reg/list", j.httpRegisterList)
	http.HandleFunc("/user/reg/image/get", j.httpRegisterImage)
	http.HandleFunc("/user/reg/delete", j.httpDeleteRegUser)

	http.HandleFunc("/user/login", j.httpUserLogin)
	http.HandleFunc("/user/info", j.httpUserInfo)
	http.HandleFunc("/user/logout", j.httpoplogout)

	http.HandleFunc("/user/alllist", j.httpUserAllList)
	http.HandleFunc("/user/list", j.httpUserList)
	http.HandleFunc("/user/userlistbyrole", j.httpUserListbyRole)
	http.HandleFunc("/user/detail", j.httpUserDetail)
	http.HandleFunc("/user/create", j.httpAddUser)
	http.HandleFunc("/user/update", j.httpUpdateUser)
	http.HandleFunc("/user/profile/update", j.httpUpdateUserProfile)
	http.HandleFunc("/user/update/avatar", j.httpUpdateUserAvatar)

	http.HandleFunc("/user/mdcid", j.httpGetMDCID)
	http.HandleFunc("/user/dmrid", j.httpGetDMRID)

	http.HandleFunc("/user/password", j.httpUpdateUserPassword)
	http.HandleFunc("/user/delete", j.httpDeleteUser)

	http.HandleFunc("/billing/info", j.httpBillingInfo)
	http.HandleFunc("/billing/packages/list", j.httpBillingPackages)
	http.HandleFunc("/billing/packages/create", j.httpBillingPackageCreate)
	http.HandleFunc("/billing/packages/update", j.httpBillingPackageUpdate)
	http.HandleFunc("/billing/packages/delete", j.httpBillingPackageDelete)
	http.HandleFunc("/billing/order/create", j.httpBillingOrderCreate)
	http.HandleFunc("/billing/order/query", j.httpBillingOrderQuery)
	http.HandleFunc("/billing/wechat/notify", j.httpBillingWechatNotify)

	//http.HandleFunc("/routes", j.httpRoutes)
	http.HandleFunc("/roles/list", j.httpGetRoles)
	http.HandleFunc("/roles/create", j.httpRole)
	http.HandleFunc("/roles/routes", j.httpGetRoutes)
	http.HandleFunc("/roles/updateroutes", j.httpSetRoutes)

	//http.HandleFunc("/area/wxuserlist", j.httpGetWeiXinUserList)
	http.HandleFunc("/operatorlog/list", j.httpOperatorLogList)

	http.HandleFunc("/health", j.httpHealth)
	http.HandleFunc("/api/homepage/sections", j.httpHomepageSections)
	http.HandleFunc("/api/homepage/announcements", j.httpHomepageAnnouncements)
	http.HandleFunc("/api/admin/homepage/sections", j.httpAdminHomepageSections)
	http.HandleFunc("/api/admin/homepage/sections/update", j.httpAdminHomepageSectionsUpdate)
	http.HandleFunc("/api/admin/homepage/sections/delete", j.httpAdminHomepageSectionsDelete)
	http.HandleFunc("/api/admin/homepage/announcements/create", j.httpAdminHomepageAnnouncementsCreate)
	http.HandleFunc("/api/admin/homepage/announcements/update", j.httpAdminHomepageAnnouncementsUpdate)
	http.HandleFunc("/api/admin/homepage/announcements/delete", j.httpAdminHomepageAnnouncementsDelete)
	http.HandleFunc("/api/admin/homepage/images/upload", j.httpAdminHomepageImageUpload)
	http.HandleFunc("/api/admin/homepage/images/list", j.httpAdminHomepageImageList)
	http.HandleFunc("/api/admin/homepage/images/delete", j.httpAdminHomepageImageDelete)

	http.Handle("/ws", websocket.Server{
		Handler:   websocket.Handler(upper),
		Handshake: allowWebsocketHandshake,
	})
	http.Handle("/ws/calls", websocket.Server{
		Handler:   websocket.Handler(j.wsCallStream),
		Handshake: allowWebsocketHandshake,
	})

	http.Handle("/", http.FileServer(http.Dir(conf.Web.Path)))

	if conf.Web.SSLCrt != "" && conf.Web.SSLKey != "" {

		err := http.ListenAndServeTLS(":"+conf.Web.Port, conf.Web.SSLCrt, conf.Web.SSLKey, nil)
		if err != nil {
			log.Println("http server start err :", err)
		}

	} else {

		err := http.ListenAndServe(":"+conf.Web.Port, nil)

		if err != nil {
			log.Println("http server start err :", err)
		}

		//err := http.ListenAndServeTLS(":"+conf.wwwport, "server.crt", "server.key", nil)

	}

	log.Println("http server on port ", conf.Web.Port)

}

type query struct {
	ID       string `json:"id"`
	User     string `json:"user"`
	Callsign string `json:"callsign"`
	SSID     uint8  `json:"ssid"`

	CountryName string `json:"country_name"`
	RegionName  string `json:"region_name"`
	ISPDomain   string `json:"isp_domain"`
	AppID       string `json:"appid"`

	GroupID  string `json:"group_id"`
	DeviceID int    `json:"device_id"`

	AreaID        string   `json:"areaid"`
	QueryType     string   `json:"querytype"`
	PhoneDistinct bool     `json:"phone_distinct"`
	QueryString   string   `json:"querys_tring"`
	OperatorID    string   `json:"operator_id"`
	Schname       string   `json:"schname"`
	Name          string   `json:"name"`
	IP            string   `json:"ip"`
	NamePhone     string   `json:"namephone"`
	Phone         string   `json:"phone"`
	Date          string   `json:"date"`
	Role          string   `json:"role"`
	Month         string   `json:"month"`
	Daterange     []string `json:"daterange"`
	UpdateTime    []string `json:"update_time"`
	FollowTime    string   `json:"follow_time"`
	CurrentArea   string   `json:"current_area"`
	Area          string   `json:"area"`
	Type          string   `json:"type"`
	EventType     string   `json:"event_type"`
	Count         int      `json:"count"`
	Limit         int      `json:"limit"`
	Page          int      `json:"page"`
	Sort          string   `json:"sort"`
	Status        string   `json:"status"`
	NotStatus     string   `json:"note_status"`
	IsOnline      bool     `json:"isonline"`
	IsDeleted     string   `json:"isdeleted"`
	Path          string   `json:"path"`
}

func queryToWhere(subquery string, q query) (string, []interface{}, string, string) {

	var s string
	var args []interface{}
	var p string
	var sort string

	if q.ID != "" {
		s = " id = ?"
		args = append(args, q.ID)
	}

	if q.OperatorID != "" {
		if s != "" {
			s = s + " and operator_id=?"
		} else {
			s = " operator_id=?"
		}
		args = append(args, q.OperatorID)
	}

	if q.CurrentArea != "" {
		if s != "" {
			s = s + " and " + subquery + "current_area=?"
		} else {
			s = " " + subquery + "current_area=?"
		}
		args = append(args, q.CurrentArea)
	}

	if q.AreaID != "" {
		if s != "" {
			s = s + " and areaid=?"
		} else {
			s = " areaid=?"
		}
		args = append(args, q.AreaID)
	}

	if q.IsDeleted != "" {
		if s != "" {
			s = s + " and isdeleted=?"
		} else {
			s = " isdeleted=?"
		}
		args = append(args, q.IsDeleted)
	}

	if q.Phone != "" {
		if s != "" {
			s = s + " and phone=?"
		} else {
			s = " phone=?"
		}
		args = append(args, q.Phone)
	}

	if q.Callsign != "" {
		q.Callsign = strings.ToUpper(q.Callsign)
		if s != "" {
			s = s + " and callsign=?"
		} else {
			s = " callsign=?"
		}
		args = append(args, q.Callsign)
	}

	if q.GroupID != "" {
		if s != "" {
			s = s + " and group_id=?"
		} else {
			s = " group_id=?"
		}
		args = append(args, q.GroupID)
	}

	if q.Role != "" {
		if s != "" {
			s = s + " and " + subquery + "roles like ?"
		} else {
			s = " " + subquery + "roles like ?"
		}
		args = append(args, "%"+q.Role+"%")
	}

	if q.Date != "" {
		if s != "" {
			s = s + " and date=?"
		} else {
			s = " date=?"
		}
		args = append(args, q.Date)
	}

	if q.Type != "" {
		if s != "" {
			s = s + " and type=?"
		} else {
			s = " type=?"
		}
		args = append(args, q.Type)
	}

	if q.Status != "" {
		if s != "" {
			s = s + " and status=?"
		} else {
			s = " status=?"
		}
		args = append(args, q.Status)
	}

	if q.NotStatus != "" {
		if s != "" {
			s = s + " and status!=?"
		} else {
			s = " status!=?"
		}
		args = append(args, q.NotStatus)
	}

	if q.Daterange != nil && len(q.Daterange) >= 2 {
		if s != "" {
			s = s + " and timestamp between ? and ?"
		} else {
			s = " timestamp between ? and ?"
		}
		args = append(args, q.Daterange[0], q.Daterange[1]+" 23:59:59")
	}

	if q.Month != "" {
		if s != "" {
			s = s + " and timestamp=?"
		} else {
			s = " timestamp=?"
		}
		args = append(args, q.Month)
	}

	if q.UpdateTime != nil && len(q.UpdateTime) >= 2 {
		if s != "" {
			s = s + " and update_time between ? and ?"
		} else {
			s = " update_time between ? and ?"
		}
		args = append(args, q.UpdateTime[0], q.UpdateTime[1]+" 23:59:59")
	}

	if q.Name != "" {
		if s != "" {
			s = s + " and (name like ?)"
		} else {
			s = " (name like ?)"
		}
		args = append(args, "%"+q.Name+"%")
	}

	if q.CountryName != "" {
		if s != "" {
			s = s + " and (country_name like ?)"
		} else {
			s = " (country_name like ?)"
		}
		args = append(args, "%"+q.CountryName+"%")
	}

	if q.RegionName != "" {
		if s != "" {
			s = s + " and (region_name like ?)"
		} else {
			s = " (region_name like ?)"
		}
		args = append(args, "%"+q.RegionName+"%")
	}

	if q.ISPDomain != "" {
		if s != "" {
			s = s + " and (isp_domain like ?)"
		} else {
			s = " (isp_domain like ?)"
		}
		args = append(args, "%"+q.ISPDomain+"%")
	}

	if q.IP != "" {
		if s != "" {
			s = s + " and (cidrip like ?)"
		} else {
			s = " (cidrip like ?)"
		}
		args = append(args, "%"+q.IP+"%")
	}

	if q.NamePhone != "" {
		if s != "" {
			s = s + " and (" + subquery + "name like ? or " + subquery + "phone like ?)"
		} else {
			s = " (" + subquery + "name like ? or " + subquery + "phone like ?)"
		}
		likeVal := "%" + q.NamePhone + "%"
		args = append(args, likeVal, likeVal)
	}

	if q.EventType != "" {
		if s != "" {
			s = s + " and (" + subquery + "event_type like ?)"
		} else {
			s = " (" + subquery + "event_type like ?)"
		}
		args = append(args, "%"+q.EventType+"%")
	}

	if q.Schname != "" {
		if s != "" {
			s = s + " and schname=?"
		} else {
			s = " schname=?"
		}
		args = append(args, q.Schname)
	}

	if s != "" {
		s = " where " + s + " "
	}

	if q.Limit > 0 && q.Page > 0 {
		p = fmt.Sprintf(" Limit %v offset %v", q.Limit, (q.Page-1)*q.Limit)
	}

	switch q.Sort {
	case "+id":
		sort = "order by id asc"
	case "-id":
		sort = "order by id desc"
	case "+name":
		sort = "order by name asc"
	case "-name":
		sort = "order by name desc"
	case "+create_time":
		sort = "order by create_time asc"
	case "-create_time":
		sort = "order by create_time desc"
	case "+update_time":
		sort = "order by update_time asc"
	case "-update_time":
		sort = "order by  update_time desc"
	case "+follow_time":
		sort = "order by follow_time asc , id asc "
	case "-follow_time":
		sort = "order by follow_time desc , id desc "
	}

	return s, args, p, sort

}