package main

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// 站点可配置文案（对应前端 nrllink-web-78ham 登录页页脚等展示位）。
// 持久化在 site_settings 表（key/value），读取时回退 YAML 默认值，
// 修改只改数据库，不触碰 YAML 文件。

type siteSetting struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt string `json:"updated_at"`
}

var (
	siteSettingKeys = []string{
		"platform_name",  // 平台/系统名称（标题）
		"logo_url",       // Logo
		"icp",            // 备案号
		"tech_support",   // 技术支持
		"copyright",      // 版权
		"login_slogan",   // 登录页副标题
		"contact_mail",   // 联系邮箱
		"contact_callsign", // 联系呼号
		"language",       // 语言
		"cs_qr_url",      // 社区二维码
	}
	siteSettingDefault = map[string]string{
		"icp":              "粤ICP备00000000号",
		"tech_support":     "技术支持：NRLLink",
		"copyright":        "Copyright © 2026 NRLLink 无线电互联",
		"login_slogan":     "无线电网络互联系统",
		"contact_mail":     "caoc@live.com",
		"contact_callsign": "BH4RPN",
	}
)

// initSiteSettingTable 建表（幂等），在 main 启动序列 updatedb() 之后调用。
func initSiteSettingTable() {
	stmt := `CREATE TABLE IF NOT EXISTS site_settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at TEXT
	)`
	if _, err := db.Exec(stmt); err != nil {
		log.Printf("[site-setting] create table err: %v", err)
	}
}

// getSiteSetting 读取单个 key；DB 无值时回退默认值（再回退 YAML）。
func getSiteSetting(key string, yamlFallback string) string {
	var value string
	err := db.QueryRow("SELECT value FROM site_settings WHERE key=?", key).Scan(&value)
	if err == nil && value != "" {
		return value
	}
	if d, ok := siteSettingDefault[key]; ok && d != "" {
		return d
	}
	return yamlFallback
}

// getAllSiteSettings 一次性返回全部可配置项（含回退后的有效值），
// 前端登录页/设置页消费此结构。
func getAllSiteSettings() []siteSetting {
	out := make([]siteSetting, 0, len(siteSettingKeys))
	for _, key := range siteSettingKeys {
		var updated string
		// 表里不存在该行时 updated 保持 ""，不影响返回
		db.QueryRow("SELECT updated_at FROM site_settings WHERE key=?", key).Scan(&updated)
		var value string
		switch key {
		case "platform_name":
			value = getSiteSetting(key, conf.SystemInfo.PlatformName)
		case "logo_url":
			value = getSiteSetting(key, conf.SystemInfo.LogoURL)
		case "icp":
			value = getSiteSetting(key, conf.Web.ICP)
		case "language":
			value = getSiteSetting(key, conf.SystemInfo.Language)
		default:
			value = getSiteSetting(key, "")
		}
		out = append(out, siteSetting{Key: key, Value: value, UpdatedAt: updated})
	}
	return out
}

// setSiteSetting 持久化单个 key（覆盖写），并刷新内存中的 YAML 默认副本
// （/platform/info 直接读 conf，这里同步一下保证两处一致）。
func setSiteSetting(key, value string) {
	value = strings.TrimSpace(value)
	_, err := db.Exec("INSERT INTO site_settings (key, value, updated_at) VALUES (?, ?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at",
		key, value, time.Now().Format("2006-01-02 15:04:05"))
	if err != nil {
		log.Printf("[site-setting] set %s err: %v", key, err)
		return
	}
	switch key {
	case "platform_name":
		conf.SystemInfo.PlatformName = value
	case "logo_url":
		conf.SystemInfo.LogoURL = value
	case "icp":
		conf.Web.ICP = value
	case "language":
		conf.SystemInfo.Language = value
	}
}

// httpSiteSettings GET 获取全部站点配置（无需登录，登录页要消费）。
func (j *jsonapi) httpSiteSettings(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	writeJSONResponseItems(w, getAllSiteSettings(), len(siteSettingKeys))
}

// httpSiteSettingsUpdate POST 更新站点配置（仅 admin）。
// 请求体：{"key":"icp","value":"..."} 或 {"settings":{"icp":"...","copyright":"..."}}
func (j *jsonapi) httpSiteSettingsUpdate(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)

	u, err := checktoken(w, req)
	if err != nil {
		return
	}
	if !checkrole(u, []string{"admin"}) {
		w.Write(ResRightErr)
		return
	}

	result, ok := readRequestBody(w, req)
	if !ok {
		return
	}

	var payload struct {
		Key      string             `json:"key"`
		Value    string             `json:"value"`
		Settings map[string]string `json:"settings"`
	}
	if err := jsonextra.Unmarshal(result, &payload); err != nil {
		log.Println("site settings update err:", err)
		w.Write(ResParmErr)
		return
	}

	upd := func(key, value string) {
		valid := false
		for _, k := range siteSettingKeys {
			if k == key {
				valid = true
				break
			}
		}
		if !valid {
			return
		}
		setSiteSetting(key, value)
	}

	if payload.Settings != nil {
		for k, v := range payload.Settings {
			upd(k, v)
		}
	} else if payload.Key != "" {
		upd(payload.Key, payload.Value)
	} else {
		w.Write(ResParmErr)
		return
	}

	addOperatorLog("site_settings", "更新站点配置成功", u)
	w.Write(ResOK)
}
