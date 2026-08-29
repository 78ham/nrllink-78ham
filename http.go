package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	// _ "net/http/pprof"
	// "github.com/jmoiron/sqlx"

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
	Name        string `json:"name"`
	LogoURL     string `json:"logourl"`
	Version     string `json:"version"`
	ICP         string `json:"icp"`
	Mail        string `json:"mail"`
	Callsign    string `json:"callsign"`
	Language    string `json:"language"`
	TechSupport string `json:"tech_support"`
	Copyright   string `json:"copyright"`
	LoginSlogan string `json:"login_slogan"`
	CSQRURL     string `json:"cs_qr_url"`
}

var (
	totalstats   = totalStats{}
	totalStatsMu sync.RWMutex
)

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

func totalStatsSnapshot() totalStats {
	totalStatsMu.RLock()
	snapshot := totalstats
	totalStatsMu.RUnlock()
	return snapshot
}

func updateTotalStats(fn func(*totalStats)) {
	totalStatsMu.Lock()
	fn(&totalstats)
	totalStatsMu.Unlock()
}

func addTotalStats(fn func(*totalStats)) {
	updateTotalStats(fn)
}

func (j *jsonapi) httpTotalStats(w http.ResponseWriter, req *http.Request) {

	stats := totalStatsSnapshot()
	stats.DevNumber = devMapLen()
	stats.OnlineDevNumber = currentOnlineDeviceCount()
	stats.UserNumber = 1000
	//totalstats.UserNumber = len(userlist)

	rescode, _ := jsonextra.Marshal(stats)

	respone := fmt.Sprintf(`{"code":20000,"data":{"items":%s}}`, rescode)

	w.Write([]byte(respone))
}

func (j *jsonapi) httpplatforminfo(w http.ResponseWriter, req *http.Request) {

	p := platforminfo{
		Name:        getSiteSetting("platform_name", conf.SystemInfo.PlatformName),
		LogoURL:     getSiteSetting("logo_url", conf.SystemInfo.LogoURL),
		Language:    getSiteSetting("language", conf.SystemInfo.Language),
		Version:     "v2.0.0",
		ICP:         getSiteSetting("icp", conf.Web.ICP),
		Mail:        getSiteSetting("contact_mail", "caoc@live.com"),
		Callsign:    getSiteSetting("contact_callsign", "BH4RPN"),
		TechSupport: getSiteSetting("tech_support", ""),
		Copyright:   getSiteSetting("copyright", ""),
		LoginSlogan: getSiteSetting("login_slogan", ""),
		CSQRURL:     getSiteSetting("cs_qr_url", ""),
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

func sethttphead(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Add("Access-Control-Allow-Headers", "x-token")
	w.Header().Set("content-type", "application/json")
}

func writeJSONResponseItems(w http.ResponseWriter, data interface{}, total int) {
	writeJSONEnvelope(w, 20000, struct {
		Total int         `json:"total,omitempty"`
		Items interface{} `json:"items"`
	}{Total: total, Items: data})
}

func writeJSONResponseItem(w http.ResponseWriter, data interface{}) {
	writeJSONEnvelope(w, 20000, struct {
		Items interface{} `json:"items"`
	}{Items: data})
}

func writeJSONResponseMessage(w http.ResponseWriter, message string, code int) {
	if code == 0 {
		code = 20000
	}
	writeJSONEnvelope(w, code, struct {
		Message string `json:"message"`
	}{Message: message})
}

func writeJSONEnvelope(w http.ResponseWriter, code int, data interface{}) {
	payload, err := jsonextra.Marshal(struct {
		Code int         `json:"code"`
		Data interface{} `json:"data"`
	}{Code: code, Data: data})
	if err != nil {
		w.Write([]byte(`{"code":20001,"data":{"message":"JSON serialization failed"}}`))
		return
	}
	w.Write(payload)
}

func readRequestBodyRaw(req *http.Request) ([]byte, error) {
	defer req.Body.Close()
	return io.ReadAll(req.Body)
}

func readRequestBody(w http.ResponseWriter, req *http.Request) ([]byte, bool) {
	body, err := readRequestBodyRaw(req)
	if err != nil {
		log.Println("read request body err:", err)
		w.Write(ResParmErr)
		return nil, false
	}
	return body, true
}

func decodeRequestJSON(w http.ResponseWriter, req *http.Request, dst interface{}) bool {
	body, ok := readRequestBody(w, req)
	if !ok {
		return false
	}
	if err := jsonextra.Unmarshal(body, dst); err != nil {
		log.Println("decode request json err:", err)
		w.Write(ResParmErr)
		return false
	}
	return true
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

	registerRoute := func(pattern string, handler func(http.ResponseWriter, *http.Request)) {
		http.HandleFunc(pattern, handler)
	}

	registerRoute("/platform/info", j.httpplatforminfo)
	registerRoute("/platform/list", j.httpplatformList)
	registerRoute("/platform/totalstats", j.httpTotalStats)
	registerRoute("/platform/site-settings", j.httpSiteSettings)
	registerRoute("/platform/site-settings/update", j.httpSiteSettingsUpdate)
	registerRoute("/server/register", j.httpRegisterServer)

	// 首页 CMS（公告/板块/图片）
	registerRoute("/homepage/sections", j.httpHomepageSections)
	registerRoute("/homepage/announcements", j.httpHomepageAnnouncements)
	registerRoute("/homepage/admin/sections", j.httpAdminHomepageSections)
	registerRoute("/homepage/admin/sections/update", j.httpAdminHomepageSectionsUpdate)
	registerRoute("/homepage/admin/sections/delete", j.httpAdminHomepageSectionsDelete)
	registerRoute("/homepage/admin/announcements/create", j.httpAdminHomepageAnnouncementsCreate)
	registerRoute("/homepage/admin/announcements/update", j.httpAdminHomepageAnnouncementsUpdate)
	registerRoute("/homepage/admin/announcements/delete", j.httpAdminHomepageAnnouncementsDelete)
	registerRoute("/homepage/admin/images/upload", j.httpAdminHomepageImageUpload)
	registerRoute("/homepage/admin/images", j.httpAdminHomepageImageList)
	registerRoute("/homepage/admin/images/delete", j.httpAdminHomepageImageDelete)

	registerRoute("/device/list", j.httpDeviceList)
	registerRoute("/device/db/list", j.httpDevicesList)
	registerRoute("/device/get", j.httpDevice)
	registerRoute("/device/qthmap", j.httpDeviceQTHs)
	registerRoute("/device/qth", j.httpDeviceQTHs)
	registerRoute("/device/qths", j.httpDeviceQTHs2)
	registerRoute("/device/qth2", j.httpDeviceQTH)
	registerRoute("/device/mydevlist", j.httpMyDeviceList)
	registerRoute("/device/update", j.httpUpdateDevice)
	registerRoute("/device/delete", j.httpDeleteDevice)
	registerRoute("/device/changegroupnrl", j.httpChangeDeviceGroupNRL)
	registerRoute("/device/query", j.httpQueryDeviceParm)
	registerRoute("/device/change", j.httpChangeDeviceParm)
	registerRoute("/device/change1w", j.httpChange1W)
	registerRoute("/device/change2w", j.httpChange2W)
	registerRoute("/device/at", j.httpDeviceAT)

	registerRoute("/group/get", j.httpGetGroup)
	registerRoute("/group/list/mini", j.httpGetGroupList)
	registerRoute("/group/list", j.httpPublicGroupList)
	registerRoute("/group/device/list", j.httpGroupDeviceList)
	registerRoute("/group/create", j.httpAddGroup)
	registerRoute("/group/update", j.httpUpdateGroup)
	registerRoute("/group/delete", j.httpDeleteGroup)
	registerRoute("/group/listnrl", j.httpAllGroupListNRL)

	registerRoute("/room/list", j.httpRoomList)

	registerRoute("/server/list", j.httpServersList)
	registerRoute("/server/create", j.httpAddServer)
	registerRoute("/server/update", j.httpUpdateServer)
	registerRoute("/server/delete", j.httpDeleteServer)

	registerRoute("/relay/list", j.httpGetRelayList)
	registerRoute("/relay/create", j.httpAddrelay)
	registerRoute("/relay/update", j.httpUpdaterelay)
	registerRoute("/relay/delete", j.httpDeleterelay)

	registerRoute("/user/reg/create", j.httpRegister)
	registerRoute("/user/reg/add", j.httpAddReg)
	registerRoute("/user/reg/update", j.httpUpdateReg)
	registerRoute("/user/reg/list", j.httpRegisterList)
	registerRoute("/user/reg/image/get", j.httpRegisterImage)
	registerRoute("/user/reg/delete", j.httpDeleteRegUser)

	registerRoute("/user/login", j.httpUserLogin)
	registerRoute("/user/info", j.httpUserInfo)
	registerRoute("/user/logout", j.httpoplogout)
	registerRoute("/user/alllist", j.httpUserAllList)
	registerRoute("/user/list", j.httpUserList)
	registerRoute("/user/userlistbyrole", j.httpUserListbyRole)
	registerRoute("/user/detail", j.httpUserDetail)
	registerRoute("/user/create", j.httpAddUser)
	registerRoute("/user/update", j.httpUpdateUser)
	registerRoute("/user/profile/update", j.httpUpdateUserProfile)
	registerRoute("/user/update/avatar", j.httpUpdateUserAvatar)
	registerRoute("/user/mdcid", j.httpGetMDCID)
	registerRoute("/user/dmrid", j.httpGetDMRID)
	registerRoute("/user/password", j.httpUpdateUserPassword)
	registerRoute("/user/delete", j.httpDeleteUser)

	registerRoute("/billing/info", j.httpBillingInfo)
	registerRoute("/billing/packages/list", j.httpBillingPackages)
	registerRoute("/billing/packages/create", j.httpBillingPackageCreate)
	registerRoute("/billing/packages/update", j.httpBillingPackageUpdate)
	registerRoute("/billing/packages/delete", j.httpBillingPackageDelete)
	registerRoute("/billing/order/create", j.httpBillingOrderCreate)
	registerRoute("/billing/order/query", j.httpBillingOrderQuery)
	registerRoute("/billing/wechat/notify", j.httpBillingWechatNotify)

	registerRoute("/roles/list", j.httpGetRoles)
	registerRoute("/roles/create", j.httpRole)
	registerRoute("/roles/routes", j.httpGetRoutes)
	registerRoute("/roles/updateroutes", j.httpSetRoutes)

	registerRoute("/operatorlog/list", j.httpOperatorLogList)

	// /api/v1/* 别名层（兼容新版前端）
	api := func(pattern string, handler func(http.ResponseWriter, *http.Request)) {
		http.HandleFunc("/api/v1"+pattern, handler)
	}
	api("/auth/login", j.httpUserLogin)
	api("/auth/me", j.httpUserInfo)
	api("/auth/logout", j.httpoplogout)
	api("/devices", j.httpDeviceList)
	api("/devices/db", j.httpDevicesList)
	api("/devices/my", j.httpMyDeviceList)
	api("/devices/qth", j.httpDeviceQTHs)
	api("/groups", j.httpPublicGroupList)
	api("/groups/mini", j.httpGetGroupList)
	api("/groups/devices", j.httpGroupDeviceList)
	api("/rooms", j.httpRoomList)
	api("/relays", j.httpGetRelayList)
	api("/servers", j.httpServersList)
	api("/users", j.httpUserList)
	api("/user/password", j.httpUpdateUserPassword)
	api("/user/create", j.httpAddUser)
	api("/user/delete", j.httpDeleteUser)
	api("/stats", j.httpTotalStats)
	api("/platform/info", j.httpplatforminfo)
	api("/platform/site-settings", j.httpSiteSettings)
	api("/platform/site-settings/update", j.httpSiteSettingsUpdate)

	//http.HandleFunc("/login", j.httplogin)
	//http.HandleFunc("/reg", j.httpreg)

	http.Handle("/ws", websocket.Server{
		Handler:   websocket.Handler(upper),
		Handshake: allowWebsocketHandshake,
	})
	http.Handle("/ws/calls", websocket.Server{
		Handler:   websocket.Handler(j.wsCallStream),
		Handshake: allowWebsocketHandshake,
	})

	http.Handle("/", http.FileServer(http.Dir(conf.Web.Path)))

	server := &http.Server{
		Addr:              ":" + conf.Web.Port,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if conf.Web.SSLCrt != "" && conf.Web.SSLKey != "" {

		err := server.ListenAndServeTLS(conf.Web.SSLCrt, conf.Web.SSLKey)
		if err != nil {
			log.Println("http server start err :", err)
		}

	} else {

		err := server.ListenAndServe()

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
