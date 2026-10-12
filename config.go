package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/mutecomm/go-sqlcipher/v4"
	yaml "gopkg.in/yaml.v3"
)

var PlatformList []Platformitem

// platformListMu 保护 PlatformList / conf.PlatformList：
// serverList/aprsget 定时刷新整体替换，与 HTTP 接口、UDP 转发路径的读取并发
var platformListMu sync.RWMutex

// setPlatformList 原子替换全局平台列表（同时维护模块级 PlatformList 与 conf.PlatformList）
func setPlatformList(list []Platformitem) {
	platformListMu.Lock()
	PlatformList = list
	conf.PlatformList = list
	platformListMu.Unlock()
}

// getPlatformList 返回当前平台列表快照；调用方不得修改返回的 slice
func getPlatformList() []Platformitem {
	platformListMu.RLock()
	defer platformListMu.RUnlock()
	return conf.PlatformList
}

type Platformitem struct {
	Name    string `yaml:"Name" json:"name"` // 对应 YAML 和 JSON 的 name 字段
	Host    string `yaml:"Host" json:"host"` // 对应 YAML 和 JSON 的 host 字段
	Port    string `yaml:"Port" json:"port"` // 对应 YAML 和 JSON 的 port 字段
	Online  int    `yaml:"Online" json:"online"`
	Total   int    `yaml:"Total" json:"total"`
	udpAddr *net.UDPAddr
}

type config struct {
	System struct {
		Port          string `yaml:"Port" json:"port"`
		Logpath       string `yaml:"LogPath" json:"log_path"`
		LicensePath   string `yaml:"LicensePath" json:"license_path"`
		DBfile        string `yaml:"DBfile" json:"dbfile"`
		IPfile        string `yaml:"IPfile" json:"ipfile"`
		CallLogPath   string `yaml:"CallLogPath" json:"calllog_path"`
		LicenseEncKey string `yaml:"LicenseEncKey" json:"license_enc_key"`
		DBEncKey      string `yaml:"DBEncKey" json:"db_enc_key"`
		FieldEncKey   string `yaml:"FieldEncKey" json:"field_enc_key"`
		FieldPepper   string `yaml:"FieldPepper" json:"field_pepper"`
	} `yaml:"System" json:"system"`

	Web struct {
		Path string `yaml:"Path" json:"path"`
		Port string `yaml:"Port" json:"port"`
		TokenKey string `yaml:"Token_Key" json:"token_key"`
		ICP    string `yaml:"ICP" json:"icp"`
		SSLCrt string `yaml:"SSLCrt" json:"ssl_crt"`
		SSLKey string `yaml:"SSLKey" json:"ssl_key"`
	} `yaml:"Web" json:"web" `

	PlatformList []Platformitem `yaml:"PlatformList" json:"platforms"`

	SystemInfo struct {
		PlatformName  string `yaml:"Name" json:"name"`
		NameShorthand string `yaml:"NameShorthand" json:"nameshorthand"`
		LogoURL       string `yaml:"LogoURL" json:"logo_url"`
		Language      string `yaml:"Language" json:"language"`
	} `yaml:"SystemInfo" json:"systeminfo"`

	OpenAI struct {
		BaseURL string `yaml:"BaseURL" json:"base_url"`
		APIKEY  string `yaml:"APIKEY" json:"api_key"`
		Engine  string `yaml:"Engine" json:"engine"`
	} `yaml:"OpenAI" json:"openai"`

	APRS struct {
		APRSServerHost string `yaml:"APRSServerHost" json:"aprs_server_host"`
		APRSServerPort string `yaml:"APRSServerPort" json:"aprs_server_port"`
		SelfAddress    string `yaml:"SelfAddress" json:"self_address"`
		SelfPort       string `yaml:"SelfPort" json:"self_port"`

		CallSign string `yaml:"CallSign" json:"callsign"`
		SSID     string `yaml:"SSID" json:"ssid"`
		Passcode int    `yaml:"Passcode" json:"passcode"`

		Latitude  float64 `yaml:"Latitude" json:"latitude"`
		Longitude float64 `yaml:"Longitude" json:"longitude"`
		Altitude  string  `yaml:"Altitude" json:"altitude"`
	} `yaml:"APRS" json:"aprs"`

	WeiXin struct {
		// signmsgurl     string
		// rechargemsgurl string
		MpAppid           string `yaml:"MpAppid" json:"mp_appid"`
		MpAppSecret       string `yaml:"MpAppSecret" json:"mp_appsecret"`
		PhoneCodeURL      string `yaml:"PhoneCodeURL" json:"phone_code_url"`
		AvatarURL         string `yaml:"AvatarURL" json:"avatar_url"`
		AccessToken       string
		AppID             string `yaml:"AppID" json:"appid"`
		AppSecret         string `yaml:"AppSecret" json:"appsecret"`
		EncodingAESKey    string `yaml:"EncodingAESKey" json:"encodingaeskey"`
		AesKey            []byte
		ClickEventMap     map[string]string `yaml:"ClickEventMap" json:"click_event_map"`
		KeywordsMap       map[string]string `yaml:"KeywordsMap" json:"keywords_map"`
		WeixinWelcome     string            `yaml:"WeixinWelcome" json:"weixin_welcome"`
		DefaultKeywords   string            `yaml:"DefaultKeywords" json:"default_keywords"`
		WeiXinAccessToken *WXBody
		WeiXinMenu        string `yaml:"WeixinMenu" json:"weixin_menu"`
		ServerURL         string `yaml:"ServerURL" json:"server_url"` //本机api url地址

		WeixinAPIURL       string `yaml:"WeixinAPIURL" json:"weixin_api_url"` //微信URL接口地址
		Wxmsgurl           string `yaml:"WxMsgURL" json:"wx_msg_url"`         //微信模板消息url
		TypePhoneCodeID    string
		TypeLoginSuccessID string

		TypeLoginFailID string

		AlarmModeID string `yaml:"AlarmModeID" json:"alarm_mode_id"` //告警通知模板ID

	} `yaml:"WeiXin" json:"weixin"`

	Billing struct {
		Enabled                  bool   `yaml:"Enabled" json:"enabled"`
		AccountExpireRecheckSecs int    `yaml:"AccountExpireRecheckSecs" json:"account_expire_recheck_secs"`
		PackageUnitPriceCents    int    `yaml:"PackageUnitPriceCents" json:"package_unit_price_cents"`
		NotifyURL                string `yaml:"NotifyURL" json:"notify_url"`
		WechatPay                struct {
			AppID          string `yaml:"AppID" json:"appid"`
			MchID          string `yaml:"MchID" json:"mch_id"`
			APIv3Key       string `yaml:"APIv3Key" json:"api_v3_key"`
			SerialNo       string `yaml:"SerialNo" json:"serial_no"`
			PrivateKeyPath string `yaml:"PrivateKeyPath" json:"private_key_path"`
			NotifyURL      string `yaml:"NotifyURL" json:"notify_url"`
			Description    string `yaml:"Description" json:"description"`
		} `yaml:"WechatPay" json:"wechat_pay"`
	} `yaml:"Billing" json:"billing"`

	Security struct {
		MinPasswordLength  int `yaml:"MinPasswordLength" json:"min_password_length"`
		MaxLoginAttempts   int `yaml:"MaxLoginAttempts" json:"max_login_attempts"`
		LockoutDuration    int `yaml:"LockoutDuration" json:"lockout_duration"`
		SessionTimeout     int `yaml:"SessionTimeout" json:"session_timeout"`
	} `yaml:"Security" json:"security"`

	Voice struct {
		MixSampleRate int `yaml:"MixSampleRate" json:"mix_sample_rate"`
	} `yaml:"Voice" json:"voice"`

	Bootstrap struct {
		DefaultAdminCallsign string `yaml:"DefaultAdminCallsign" json:"default_admin_callsign"`
	} `yaml:"bootstrap" json:"bootstrap"`
}

var conf = &config{}

func (c *config) init() {

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		log.Printf("get config filepath err #%v ", err)
		os.Exit(1)
	}

	confpath := dir + "/udphub.yaml"

	cc := flag.String("c", confpath, "config file path and name")
	oo := flag.String("o", "", "print config content to stdout and exit , yaml or json format")
	backupOut := flag.String("backup", "", "backup database snapshot to target file and exit")

	flag.Parse()

	if *cc != "" {
		confpath = *cc
	}

	yamlFile, err := os.ReadFile(confpath)

	if err != nil {
		log.Printf("udphub.yaml open err #%v ", err)
		os.Exit(1)

	}
	err = yaml.Unmarshal(yamlFile, conf)

	if err != nil {
		log.Fatalf("Unmarshal: %v \n %s", err, yamlFile)
	}

	// c.Parm.iDCfilterIPMap = make(map[uint32]bool, 0)
	// for _, v := range c.Parm.IDCfilterIP {
	// 	c.Parm.iDCfilterIPMap[ipstrToUInt32(v)] = true
	// }

	if *oo != "" {
		switch *oo {
		case "json":
			j, _ := jsonextra.MarshalIndent(conf, "", "    ")
			fmt.Println(string(j))
		case "yaml":
			j, _ := yaml.Marshal(conf)
			fmt.Println(string(j))

		}
		os.Exit(0)
	}

	PlatformList = conf.PlatformList

	if conf.Security.MinPasswordLength == 0 {
		conf.Security.MinPasswordLength = 8
	}
	if conf.Security.MaxLoginAttempts == 0 {
		conf.Security.MaxLoginAttempts = 10
	}
	if conf.Security.LockoutDuration == 0 {
		conf.Security.LockoutDuration = 900
	}
	if conf.Security.SessionTimeout == 0 {
		conf.Security.SessionTimeout = 86400
	}
	if conf.Voice.MixSampleRate == 0 {
		conf.Voice.MixSampleRate = 16000
	}
	if conf.Bootstrap.DefaultAdminCallsign == "" {
		conf.Bootstrap.DefaultAdminCallsign = "NOCALL"
	}

	// 环境变量覆盖，优先级高于配置文件
	if v := os.Getenv("NRL_IPFILE"); v != "" {
		conf.System.IPfile = v
	}
	if v := os.Getenv("NRL_WEBPATH"); v != "" {
		conf.Web.Path = v
	}
	if v := os.Getenv("NRL_TOKEN_KEY"); v != "" {
		conf.Web.TokenKey = v
	}
	if v := os.Getenv("NRL_LICENSE_KEY"); v != "" {
		conf.System.LicenseEncKey = v
	}
	if v := os.Getenv("NRL_DB_KEY"); v != "" {
		conf.System.DBEncKey = v
	}
	if v := os.Getenv("NRL_FIELD_KEY"); v != "" {
		conf.System.FieldEncKey = v
	}
	if v := os.Getenv("NRL_FIELD_PEPPER"); v != "" {
		conf.System.FieldPepper = v
	}

	if conf.System.LicenseEncKey != "" {
		licenseAESKey = deriveKey(conf.System.LicenseEncKey)
	}
	if conf.System.FieldEncKey != "" {
		fieldAESKey = deriveKey(conf.System.FieldEncKey)
	}
	if conf.System.FieldPepper != "" {
		fieldPepperKey = []byte(conf.System.FieldPepper)
	} else {
		fieldPepperKey = []byte("nrllink_default_pepper_key_2026")
	}

	// 命令行触发数据库快速备份
	if *backupOut != "" {
		backupDB := getDB()
		target := filepath.ToSlash(*backupOut)
		if err := os.MkdirAll(filepath.Dir(*backupOut), 0755); err != nil {
			log.Fatalf("创建备份目录失败: %v", err)
		}
		_ = os.Remove(*backupOut)
		_, err := backupDB.Exec(fmt.Sprintf("VACUUM INTO '%s';", target))
		backupDB.Close()
		if err != nil {
			log.Fatalf("执行数据库热备份失败: %v", err)
		}
		log.Printf("[backup] 数据库快照备份成功: %s", *backupOut)
		os.Exit(0)
	}
}

// Exist 判断文件存在
func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
	// || os.IsExist(err)
}

var db *sql.DB

func migrateToSQLCipher(dbFile string, key string) error {
	backupFile := fmt.Sprintf("%s.bak.%d", dbFile, time.Now().Unix())
	if data, err := os.ReadFile(dbFile); err == nil {
		_ = os.WriteFile(backupFile, data, 0644)
	}

	tempFile := dbFile + ".migrated"
	_ = os.Remove(tempFile)

	dbPlain, err := sql.Open("sqlite3", dbFile)
	if err != nil {
		return err
	}
	defer dbPlain.Close()

	attachSQL := fmt.Sprintf("ATTACH DATABASE '%s' AS encrypted KEY '%s';", filepath.ToSlash(tempFile), key)
	if _, err := dbPlain.Exec(attachSQL); err != nil {
		return fmt.Errorf("attach encrypted db failed: %w", err)
	}

	if _, err := dbPlain.Exec("SELECT sqlcipher_export('encrypted');"); err != nil {
		return fmt.Errorf("sqlcipher_export failed: %w", err)
	}

	if _, err := dbPlain.Exec("DETACH DATABASE encrypted;"); err != nil {
		return fmt.Errorf("detach encrypted db failed: %w", err)
	}

	dbPlain.Close()

	if err := os.Rename(tempFile, dbFile); err != nil {
		if data, err := os.ReadFile(tempFile); err == nil {
			_ = os.WriteFile(dbFile, data, 0644)
			_ = os.Remove(tempFile)
		}
	}

	log.Printf("[sqlcipher] 已成功将明文 SQLite 数据库迁移为 SQLCipher 加密数据库，原库备份至: %s", backupFile)
	return nil
}

func getDB() *sql.DB {

	if envDBfile := os.Getenv("NRL_DBFILE"); envDBfile != "" {
		conf.System.DBfile = envDBfile
	}

	if err := os.MkdirAll(filepath.Dir(conf.System.DBfile), 0755); err != nil {
		log.Fatalf("create db dir err: %v", err)
	}

	if !Exist(conf.System.DBfile) {
		log.Println("检测到首次部署，将自动创建数据库")
	} else if conf.System.DBEncKey != "" {
		// 探测已有数据库是否为旧明文数据库，若是则自动执行 SQLCipher 迁移
		probeDSN := "file:" + conf.System.DBfile + "?_pragma_key=" + conf.System.DBEncKey
		probeDB, err := sql.Open("sqlite3", probeDSN)
		if err == nil {
			var test string
			err = probeDB.QueryRow("PRAGMA quick_check").Scan(&test)
			probeDB.Close()
			if err != nil || test != "ok" {
				// 带 key 打开失败，探测是否为明文库
				plainDB, plainErr := sql.Open("sqlite3", conf.System.DBfile)
				if plainErr == nil {
					var plainTest string
					if plainDB.QueryRow("PRAGMA quick_check").Scan(&plainTest) == nil && plainTest == "ok" {
						plainDB.Close()
						if err := migrateToSQLCipher(conf.System.DBfile, conf.System.DBEncKey); err != nil {
							log.Fatalf("自动迁移到加密数据库失败: %v", err)
						}
					} else {
						plainDB.Close()
					}
				}
			}
		}
	}

	var err error

	dsn := "file:" + conf.System.DBfile + "?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL"
	if conf.System.DBEncKey != "" {
		dsn += "&_pragma_key=" + conf.System.DBEncKey + "&_pragma_cipher_page_size=4096"
	}

	// WAL：读写不再互斥；synchronous=NORMAL：WAL 下安全且避免每次 commit fsync
	db, err = sql.Open("sqlite3", dsn)

	if err != nil {
		log.Fatal(err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal(err)
	}

	var integrity string
	if err = db.QueryRow("PRAGMA quick_check").Scan(&integrity); err != nil || integrity != "ok" {
		log.Fatalf("数据库文件损坏: %v，请人工介入，程序不会自动删除或重建数据库", integrity)
	}

	// WAL 下读连接可并行；写并发由 _busy_timeout 兜底
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	return db
}

func updatedb() {
	sqlStatements := []string{
		//"ALTER TABLE devices ADD COLUMN callsign TEXT DEFAULT '';",
		"ALTER TABLE devices ADD COLUMN priority INTEGER DEFAULT 100;",
		//"ALTER TABLE devices RENAME COLUMN cpuid TO dmrid;",

		"ALTER TABLE users ADD COLUMN mdcid string DEFAULT '';",

		"ALTER TABLE devices ADD COLUMN dmrid INTEGER DEFAULT 0;",
		"ALTER TABLE users ADD COLUMN dmrid INTEGER DEFAULT 0;",
		"ALTER TABLE users ADD COLUMN expire_time TEXT DEFAULT '';",
		//"update devices set dmrid='';",
		//"ALTER TABLE public_groups ADD COLUMN allow_callsign_ssid TEXT DEFAULT '';",
		//"CREATE UNIQUE INDEX idx_ssid_callsign ON devices (ssid, callsign);",
		//"CREATE UNIQUE INDEX idx_name_unique ON public_groups(name);",

		`CREATE TABLE IF NOT EXISTS billing_packages (
			id INTEGER UNIQUE,
			name TEXT,
			months INTEGER,
			unit_price_cents INTEGER,
			price_cents INTEGER,
			status INTEGER,
			note TEXT,
			create_time TEXT,
			update_time TEXT,
			PRIMARY KEY("id" AUTOINCREMENT)
		);`,
		`CREATE TABLE IF NOT EXISTS billing_orders (
			id INTEGER UNIQUE,
			out_trade_no TEXT UNIQUE,
			user_id INTEGER,
			callsign TEXT,
			package_id INTEGER,
			months INTEGER,
			amount_cents INTEGER,
			status TEXT,
			prepay_id TEXT,
			code_url TEXT,
			transaction_id TEXT,
			payer_openid TEXT,
			paid_at TEXT,
			expire_before TEXT,
			expire_after TEXT,
			raw_notify TEXT,
			create_time TEXT,
			update_time TEXT,
			PRIMARY KEY("id" AUTOINCREMENT)
		);`,

		"DELETE FROM users WHERE id NOT IN ( SELECT MIN(id) FROM users GROUP BY phone );",
		"DROP INDEX idx_phone_unique ;",
		"DROP INDEX idx_callsign_unique",
		"CREATE UNIQUE INDEX idx_users_phone_unique ON users(phone);",
		"CREATE UNIQUE INDEX idx_users_callsign_unique ON users(callsign);",

		// —— 数据库重构（追加迁移，只允许在末尾追加，严禁修改以上既有语句文本）——
		"ALTER TABLE users ADD COLUMN must_change_pwd INTEGER DEFAULT 0;",
		"UPDATE users SET must_change_pwd=1, routes='' WHERE routes='MUST_CHANGE_PWD';",
		"ALTER TABLE users ADD COLUMN mdcid TEXT DEFAULT '';",
		"ALTER TABLE users ADD COLUMN dmrid INTEGER DEFAULT 0 ;",
		"ALTER TABLE devices ADD COLUMN priority INTEGER DEFAULT 100 ;",

		// 补齐 devices.rf_type（旧库无此列会导致 initAllDevList 查询失败）
		"ALTER TABLE devices ADD COLUMN rf_type INTEGER DEFAULT 0;",
		// 清理 users 表 NULL 值（扫描到 string/int 会报错，旧库补默认值）
		"UPDATE users SET gird=COALESCE(gird,''), birthday=COALESCE(birthday,''), sex=COALESCE(sex,0), avatar=COALESCE(avatar,''), address=COALESCE(address,''), introduction=COALESCE(introduction,''), last_login_time=COALESCE(last_login_time,''), openid=COALESCE(openid,''), nickname=COALESCE(nickname,''), pid=COALESCE(pid,''), last_login_ip=COALESCE(last_login_ip,''), expire_time=COALESCE(expire_time,''), mdcid=COALESCE(mdcid,''), dmrid=COALESCE(dmrid,0);",
		"ALTER TABLE users ADD COLUMN lock_until INTEGER DEFAULT 0;",
		"ALTER TABLE users ADD COLUMN token_version INTEGER DEFAULT 1;",
		"ALTER TABLE users ADD COLUMN phone_hash TEXT DEFAULT '';",
		"ALTER TABLE registers ADD COLUMN phone_hash TEXT DEFAULT '';",
		"CREATE INDEX IF NOT EXISTS idx_users_phone_hash ON users(phone_hash);",
		"CREATE INDEX IF NOT EXISTS idx_registers_phone_hash ON registers(phone_hash);",
		"ALTER TABLE users ADD COLUMN totp_secret TEXT DEFAULT '';",
		"ALTER TABLE users ADD COLUMN totp_enabled INTEGER DEFAULT 0;",
		`CREATE TABLE IF NOT EXISTS user_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			credential_id BLOB NOT NULL,
			public_key BLOB NOT NULL,
			attestation_type TEXT DEFAULT '',
			aaguid BLOB,
			sign_count INTEGER DEFAULT 0,
			name TEXT DEFAULT '',
			create_time TEXT,
			last_used_time TEXT
		);`,
		"CREATE INDEX IF NOT EXISTS idx_user_credentials_user_id ON user_credentials(user_id);",
	}

	// 记录已执行的迁移语句,保证每条只执行一次,避免每次启动重复执行
	// (尤其是 DELETE/DROP 等破坏性或一次性语句),也不再刷无意义的错误日志。
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (stmt TEXT PRIMARY KEY, applied_at TEXT)`); err != nil {
		log.Printf("create schema_migrations table err: %v", err)
		return
	}

	for _, stmt := range sqlStatements {
		var applied int
		if err := db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE stmt=?`, stmt).Scan(&applied); err != nil {
			log.Printf("check migration err: %v, stmt: %s", err, stmt)
			continue
		}
		if applied > 0 {
			continue
		}

		log.Printf("Executing SQL: %s\n", stmt)
		_, err := db.Exec(stmt)
		if err != nil {
			// 执行失败(如列已存在/索引不存在)也记录为已处理,避免每次启动重试刷日志。
			log.Printf("Error executing statement: %s\nError: %v (recorded as applied, will not retry)\n", stmt, err)
		}
		if _, err := db.Exec(`INSERT OR IGNORE INTO schema_migrations(stmt, applied_at) VALUES(?, CURRENT_TIMESTAMP)`, stmt); err != nil {
			log.Printf("record migration err: %v, stmt: %s", err, stmt)
		}
	}

	backfillPhoneHash()
}

func backfillPhoneHash() {
	rows, err := db.Query("SELECT id, phone FROM users WHERE (phone_hash IS NULL OR phone_hash = '') AND phone != ''")
	if err == nil {
		defer rows.Close()
		type uRecord struct {
			id    int
			phone string
		}
		var updates []uRecord
		for rows.Next() {
			var r uRecord
			if rows.Scan(&r.id, &r.phone) == nil {
				updates = append(updates, r)
			}
		}
		for _, u := range updates {
			plainPhone := decryptField(fieldAESKey, u.phone)
			hash := hashIndex(fieldPepperKey, plainPhone)
			_, _ = db.Exec("UPDATE users SET phone_hash=? WHERE id=?", hash, u.id)
		}
	}

	rRows, rErr := db.Query("SELECT id, phone FROM registers WHERE (phone_hash IS NULL OR phone_hash = '') AND phone != ''")
	if rErr == nil {
		defer rRows.Close()
		type rRecord struct {
			id    int
			phone string
		}
		var rUpdates []rRecord
		for rRows.Next() {
			var r rRecord
			if rRows.Scan(&r.id, &r.phone) == nil {
				rUpdates = append(rUpdates, r)
			}
		}
		for _, item := range rUpdates {
			plainPhone := decryptField(fieldAESKey, item.phone)
			hash := hashIndex(fieldPepperKey, plainPhone)
			_, _ = db.Exec("UPDATE registers SET phone_hash=? WHERE id=?", hash, item.id)
		}
	}
}
