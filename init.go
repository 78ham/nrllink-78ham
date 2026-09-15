package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ensureBootstrap 在 updatedb() 之后、initAllUserList() 之前调用。
// 每次启动先清除发行包自带的旧默认管理员账号（purgeLegacyAdmins），
// 再检测是否为首次启动（无用户表或无 admin 角色用户），若是则：
//  1. 创建 admin 角色
//  2. 创建默认 admin 用户（callsign 取配置，缺省 NOCALL, roles=admin, must_change_pwd=1）
//  3. 打印随机密码到 stdout，并写入 bootstrapped_at 元数据
//
// 注意：DDL 已由 main 中的 execDDL() 前置执行，此处不再调用。
func ensureBootstrap() {
	// purgeLegacyAdmins 过渡保留，一个版本周期后移除。
	purgeLegacyAdmins()

	if isBootstrapped() {
		return
	}

	log.Println("[bootstrap] 检测到首次启动，正在初始化数据库...")

	seedRoles()
	if seedAdmin() {
		setMeta("bootstrapped_at", time.Now().Format("2006-01-02 15:04:05"))
		log.Println("[bootstrap] 数据库初始化完成")
	} else {
		// seed 失败不写 bootstrapped_at，下次启动将重试引导流程，
		// 避免出现“无管理员但已标记为已引导”的死锁状态。
		log.Println("[bootstrap] 默认管理员创建失败，未写入引导标记，下次启动将重试")
	}
}

// isBootstrapped 先查 meta 表的 bootstrapped_at 标记；无标记时回退旧判定：
// 存在 admin 用户则仅补写元数据（不触发 seed），否则视为未初始化。
func isBootstrapped() bool {
	if _, ok := getMeta("bootstrapped_at"); ok {
		return true
	}

	var adminCount int
	err := db.QueryRow("SELECT count(*) FROM users WHERE roles LIKE '%admin%'").Scan(&adminCount)
	if err != nil {
		return false
	}
	if adminCount > 0 {
		setMeta("bootstrapped_at", time.Now().Format("2006-01-02 15:04:05"))
		return true
	}
	return false
}

// getMeta 读取 meta 表键值，表不存在/无记录时返回 "", false。
func getMeta(key string) (string, bool) {
	var value string
	err := db.QueryRow("SELECT value FROM meta WHERE key=?", key).Scan(&value)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("[bootstrap] get meta %s error: %v", key, err)
		}
		return "", false
	}
	return value, true
}

// setMeta 写入 meta 表键值，已存在则覆盖。
func setMeta(key, value string) {
	_, err := db.Exec("INSERT INTO meta (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, value)
	if err != nil {
		log.Printf("[bootstrap] set meta %s error: %v", key, err)
	}
}

// purgeLegacyAdmins 删除发行包自带 udphub.sqlite3 中的旧默认管理员账号。
// 该账号（callsign=NOCALL, phone=18900000000）自 2022 年随仓库分发，密码无从考证，
// 且会使 isBootstrapped 误判为已初始化、跳过 seedAdmin，导致新部署既拿不到默认
// 管理员、又遗留一个来历不明的管理员账号。每次启动都执行（幂等），以同时修复
// 已部署的旧数据库；删除后若无任何管理员，seedAdmin 会创建新的随机密码管理员。
func purgeLegacyAdmins() {
	var tblCount int
	err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tblCount)
	if err != nil || tblCount == 0 {
		return
	}

	res, err := db.Exec("DELETE FROM users WHERE callsign='NOCALL' AND phone='18900000000' AND roles LIKE '%admin%'")
	if err != nil {
		log.Printf("[bootstrap] purge legacy admin error: %v", err)
		return
	}
	if n, err := res.RowsAffected(); err == nil && n > 0 {
		log.Printf("[bootstrap] 已移除发行包自带的旧默认管理员账号 (callsign=NOCALL, phone=18900000000)，共 %d 个", n)
	}
}

func execDDL() {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS "users" (
			"id"	INTEGER UNIQUE,
			"name"	TEXT,
			"callsign"	TEXT,
			"gird"	TEXT DEFAULT '',
			"phone"	TEXT,
			"password"	TEXT,
			"birthday"	TEXT DEFAULT '',
			"sex"	BLOB DEFAULT 0,
			"avatar"	TEXT DEFAULT '',
			"address"	TEXT DEFAULT '',
			"roles"	TEXT,
			"introduction"	TEXT DEFAULT '',
			"alarm_msg"	BLOB,
			"status"	INTEGER,
			"update_time"	TEXT,
			"last_login_time"	TEXT DEFAULT '',
			"login_err_times"	INTEGER,
			"create_time"	TEXT,
			"openid"	TEXT DEFAULT '',
			"nickname"	TEXT DEFAULT '',
			"pid"	TEXT DEFAULT '',
			"last_login_ip"	TEXT DEFAULT '',
			"expire_time"	TEXT DEFAULT '',
			"routes"	TEXT,
			"mdcid"	TEXT DEFAULT '',
			"dmrid"	INTEGER DEFAULT 0,
			"must_change_pwd"	INTEGER DEFAULT 0,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "devices" (
			"id"	INTEGER UNIQUE,
			"name"	TEXT,
			"dmrid"	TEXT,
			"callsign" TEXT,
			"ssid" INTEGER,
			"password"	TEXT,
			"gird"	TEXT,
			"dev_type"	INTEGER,
			"dev_model"	INTEGER,
			"group_id"	INTEGER,
			"status"	INTEGER,
			"is_certed"	BLOB,
			"chan_name"	TEXT,
			"online_time"	TEXT,
			"create_time"	TEXT,
			"update_time"	TEXT,
			"note"	TEXT,
			"priority"	INTEGER DEFAULT 100,
			"rf_type"	INTEGER DEFAULT 0,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "public_groups" (
			"id"	INTEGER UNIQUE,
			"name"	TEXT,
			"type"	INTEGER,
			"callsign"	TEXT,
			"password"	TEXT,
			"allow_dmrid"	TEXT,
			"allow_callsign_ssid" TEXT,
			"ower_id"	INTEGER,
			"devlist"	TEXT,
			"master_server"	INTEGER,
			"slave_server"	INTEGER,
			"status"	INTEGER,
			"create_time"	TEXT,
			"update_time"	TEXT,
			"note"	TEXT,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "servers" (
			"id"	INTEGER UNIQUE,
			"name"	TEXT,
			"server_type"	INTEGER,
			"join_key"	TEXT,
			"cpu_type"	TEXT,
			"mem_size"	TEXT,
			"input_rate"	INTEGER,
			"output_rate"	INTEGER,
			"netcard"	TEXT,
			"ip_type"	INTEGER,
			"ip_addr"	TEXT,
			"dns_name"	TEXT,
			"group_list"	INTEGER,
			"ower_id"	TEXT,
			"ower_callsign"	TEXT,
			"is_online"	NUMERIC,
			"status"	INTEGER,
			"create_time"	TEXT,
			"update_time"	TEXT,
			"note"	TEXT,
			"udp_port"	INTEGER,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "roles" (
			"id"	INTEGER UNIQUE,
			"name_key"	TEXT,
			"name"	TEXT,
			"description"	TEXT,
			"routes"	TEXT,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "operator_log" (
			"id"	INTEGER UNIQUE,
			"timestamp"	TEXT,
			"content"	TEXT,
			"event_type"	TEXT,
			"operator"	TEXT,
			"operator_id"	INTEGER,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "relay" (
			"id"	INTEGER UNIQUE,
			"name"	TEXT,
			"up_freq"	TEXT,
			"down_freq"	TEXT,
			"send_ctss"	TEXT,
			"recive_ctss"	TEXT,
			"ower_callsign"	TEXT,
			"create_time"	TEXT,
			"update_time"	TEXT,
			"status"	INTEGER,
			"note"	TEXT,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "routes" (
			"id"	INTEGER UNIQUE,
			"routes"	TEXT,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "registers" (
			"id"	INTEGER UNIQUE,
			"name"	TEXT,
			"callsign"	TEXT UNIQUE,
			"phone"	TEXT,
			"password"	TEXT,
			"address"	TEXT,
			"mail"	TEXT,
			"birthday"	TEXT,
			"sex"	INTEGER,
			"op_cert_path"	TEXT,
			"license_path"	TEXT,
			"status"	INTEGER DEFAULT 1,
			"create_time"	TEXT,
			"update_time"	TEXT,
			"note"	TEXT,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "msglog" (
			"id"	INTEGER UNIQUE,
			"timestamp"	TEXT,
			"content"	TEXT,
			"msgtype"	TEXT,
			"callsign"	TEXT,
			"ssid"	INTEGER,
			"src"	TEXT,
			"dest"	TEXT,
			"group_id"	INTEGER,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "dmrid" (
			"id"	INTEGER UNIQUE,
			"dmrid"	INTEGER,
			"callsign"	TEXT,
			"ssid"	INTEGER,
			"note"	TEXT,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "aprs_config" (
			"id"	INTEGER UNIQUE,
			"host"	TEXT,
			"port"	INTEGER,
			"callsign"	TEXT,
			"ssid"	TEXT,
			"passcode"	INTEGER,
			"enabled"	INTEGER,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "call_log" (
			"id"	INTEGER UNIQUE,
			"room_key"	TEXT,
			"room_name"	TEXT,
			"callsign"	TEXT,
			"ssid"	INTEGER,
			"started_at"	TEXT,
			"duration_ms"	INTEGER,
			"create_time"	TEXT,
			PRIMARY KEY("id" AUTOINCREMENT)
		)`,
		`CREATE TABLE IF NOT EXISTS "meta" ("key" TEXT PRIMARY KEY, "value" TEXT)`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			log.Printf("[bootstrap] DDL error: %v\n  SQL: %s", err, stmt)
		}
	}

	indexes := []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_users_callsign_unique ON users(callsign)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_unique ON users(phone)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_ssid_callsign ON devices(ssid, callsign)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_public_groups_name ON public_groups(name)",
	}
	for _, stmt := range indexes {
		if _, err := db.Exec(stmt); err != nil {
			log.Printf("[bootstrap] index error: %v\n  SQL: %s", err, stmt)
		}
	}
}

// initRolesTable 修复 roles 表的历史遗留问题，必须在 ensureBootstrap() 之前调用。
//
// 背景：DDL 早期把列名拼成了 routess，而查询侧按 routes 读取；seedRoles 插入时
// 又没给 routes 赋值，导致 NULL 写入。两者叠加会让 getRoles 在 rows.Scan 时
// 报 "converting NULL to string is unsupported"，整行被 continue 掉，接口返回
// 空数组，前端角色下拉框就显示"无数据"。
//
// 这里做三件事（全部幂等）：
//  1. 老库若只有 routess 列，重命名为 routes；
//  2. 补齐缺失的 routes 列（极端情况）；
//  3. 把 routes 的 NULL 值刷成空串，避免 Scan 失败。
func initRolesTable() {
	var tblCount int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='roles'").Scan(&tblCount); err != nil || tblCount == 0 {
		return
	}

	cols := map[string]bool{}
	rows, err := db.Query("PRAGMA table_info(roles)")
	if err != nil {
		log.Printf("[roles] read table_info error: %v", err)
		return
	}
	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notNull    int
			defaultVal any
			pk         int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &defaultVal, &pk); err != nil {
			continue
		}
		cols[name] = true
	}
	rows.Close()

	if cols["routess"] && !cols["routes"] {
		if _, err := db.Exec("ALTER TABLE roles RENAME COLUMN routess TO routes"); err != nil {
			log.Printf("[roles] rename routess -> routes error: %v", err)
		} else {
			log.Println("[roles] 已将 roles.routess 重命名为 roles.routes")
			cols["routes"] = true
		}
	}

	if !cols["routes"] {
		if _, err := db.Exec("ALTER TABLE roles ADD COLUMN routes TEXT DEFAULT ''"); err != nil {
			log.Printf("[roles] add routes column error: %v", err)
		} else {
			log.Println("[roles] 已补齐 roles.routes 列")
		}
	}

	if _, err := db.Exec("UPDATE roles SET routes='' WHERE routes IS NULL"); err != nil {
		log.Printf("[roles] fill NULL routes error: %v", err)
	}
}

// initRegistersTable 修复用户注册表的历史遗留问题，必须在 ensureBootstrap() 之前调用。
//
// 背景：代码里增删改查用的都是 registers 表，而 DDL 重构时把建表语句写成了
// user_reg，列结构（image、没有 address/mail/op_cert_path/license_path）
// 与代码完全对不上。结果是 POST /user/reg/create 在 db.Prepare 阶段就报
// "no such table: registers"，注册记录一条都写不进去，后台注册列表永远为空。
//
// 这里做三件事（全部幂等）：
//  1. 老库若只有 user_reg（历史误建）则重命名为 registers；
//  2. 补齐代码需要的所有列；
//  3. 把文本列的 NULL 刷成空串，避免 Scan 失败。
func initRegistersTable() {
	tableExists := func(name string) bool {
		var n int
		if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", name).Scan(&n); err != nil {
			return false
		}
		return n > 0
	}

	hasRegisters := tableExists("registers")
	hasLegacy := tableExists("user_reg")

	// 老部署若只有误建的 user_reg，直接整表改名，数据不丢。
	if !hasRegisters && hasLegacy {
		if _, err := db.Exec("ALTER TABLE user_reg RENAME TO registers"); err != nil {
			log.Printf("[registers] rename user_reg -> registers error: %v", err)
			return
		}
		log.Println("[registers] 已将历史误建的 user_reg 重命名为 registers")
		hasRegisters = true
		hasLegacy = false
	}

	if !hasRegisters {
		return
	}

	cols := map[string]bool{}
	rows, err := db.Query("PRAGMA table_info(registers)")
	if err != nil {
		log.Printf("[registers] read table_info error: %v", err)
		return
	}
	for rows.Next() {
		var (
			cid        int
			name       string
			ctype      string
			notNull    int
			defaultVal any
			pk         int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &defaultVal, &pk); err != nil {
			continue
		}
		cols[name] = true
	}
	rows.Close()

	needed := []struct{ name, decl string }{
		{"address", "TEXT DEFAULT ''"},
		{"mail", "TEXT DEFAULT ''"},
		{"birthday", "TEXT DEFAULT ''"},
		{"sex", "INTEGER DEFAULT 0"},
		{"op_cert_path", "TEXT DEFAULT ''"},
		{"license_path", "TEXT DEFAULT ''"},
		{"status", "INTEGER DEFAULT 1"},
		{"create_time", "TEXT DEFAULT ''"},
		{"update_time", "TEXT DEFAULT ''"},
		{"note", "TEXT DEFAULT ''"},
	}
	for _, c := range needed {
		if cols[c.name] {
			continue
		}
		if _, err := db.Exec("ALTER TABLE registers ADD COLUMN " + c.name + " " + c.decl); err != nil {
			log.Printf("[registers] add column %s error: %v", c.name, err)
		} else {
			log.Printf("[registers] 已补齐 registers.%s 列", c.name)
		}
	}

	// 两个表同时存在的场景（老库误建 user_reg + 新版 execDDL 新建空 registers）：
	// 把 user_reg 里已有数据搬到 registers，否则老注册记录在后台列表里会消失。
	if hasLegacy {
		var legacyCount, newCount int
		_ = db.QueryRow("SELECT count(*) FROM user_reg").Scan(&legacyCount)
		_ = db.QueryRow("SELECT count(*) FROM registers").Scan(&newCount)
		if legacyCount > 0 && newCount == 0 {
			if _, err := db.Exec(`INSERT INTO registers
				(name, callsign, phone, password, op_cert_path, license_path, status, create_time, update_time, note)
				SELECT COALESCE(name,''), COALESCE(callsign,''), COALESCE(phone,''), COALESCE(password,''),
					'', '', COALESCE(status,1), COALESCE(create_time,''), COALESCE(update_time,''), COALESCE(note,'')
				FROM user_reg`); err != nil {
				log.Printf("[registers] migrate user_reg -> registers error: %v", err)
			} else {
				log.Printf("[registers] 已从 user_reg 搬迁 %d 条历史注册记录", legacyCount)
			}
		}
	}

	if _, err := db.Exec(`UPDATE registers SET
		name=COALESCE(name,''), callsign=COALESCE(callsign,''), phone=COALESCE(phone,''),
		password=COALESCE(password,''), address=COALESCE(address,''), mail=COALESCE(mail,''),
		birthday=COALESCE(birthday,''), sex=COALESCE(sex,0),
		op_cert_path=COALESCE(op_cert_path,''), license_path=COALESCE(license_path,''),
		status=COALESCE(status,1), create_time=COALESCE(create_time,''),
		update_time=COALESCE(update_time,''), note=COALESCE(note,'')`); err != nil {
		log.Printf("[registers] fill NULL error: %v", err)
	}

	// 呼号唯一：重复提交同一呼号时 createRegUser 会走"更新证照路径"分支，
	// 而不是插入重复记录。老库若已有重复数据则该索引创建失败，仅告警不影响启动。
	if _, err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_registers_callsign_unique ON registers(callsign)"); err != nil {
		log.Printf("[registers] create callsign unique index skipped: %v", err)
	}
}

func seedRoles() {
	roleDefs := []struct {
		Key         string
		Name        string
		Description string
	}{
		{"admin", "管理员", "系统管理员，拥有全部权限"},
		{"ham", "HAM用户", "业余无线电用户"},
		{"view", "观察员", "只读权限"},
	}

	for _, r := range roleDefs {
		var count int
		db.QueryRow("SELECT count(*) FROM roles WHERE name_key=?", r.Key).Scan(&count)
		if count > 0 {
			continue
		}
		_, err := db.Exec("INSERT INTO roles (name_key, name, description, routes) VALUES (?, ?, ?, ' ')",
			r.Key, r.Name, r.Description)
		if err != nil {
			log.Printf("[bootstrap] seed role %s error: %v", r.Key, err)
		}
	}
}

func randPassword(length int) (string, error) {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[n.Int64()]
	}
	return string(result), nil
}

// seedAdmin 创建默认管理员。返回 true 表示库中已存在管理员（本来就存在或创建成功），
// 返回 false 表示创建失败，调用方不得写入引导标记。
func seedAdmin() bool {
	var count int
	err := db.QueryRow("SELECT count(*) FROM users WHERE roles LIKE '%admin%'").Scan(&count)
	if err != nil {
		log.Printf("[bootstrap] check admin count error: %v", err)
		return false
	}
	if count > 0 {
		return true
	}

	password, err := randPassword(16)
	if err != nil {
		log.Printf("[bootstrap] generate password failed: %v", err)
		return false
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("[bootstrap] hash password failed: %v", err)
		return false
	}

	callsign := conf.Bootstrap.DefaultAdminCallsign
	phone := "00000000000"

	// 冲突检测：预留手机号或配置呼号已被占用时改用随机值，避免唯一索引冲突导致创建失败。
	var phoneCount int
	if err := db.QueryRow("SELECT count(*) FROM users WHERE phone=?", phone).Scan(&phoneCount); err != nil {
		log.Printf("[bootstrap] check phone count error: %v", err)
		return false
	}
	var callsignCount int
	if err := db.QueryRow("SELECT count(*) FROM users WHERE callsign=?", callsign).Scan(&callsignCount); err != nil {
		log.Printf("[bootstrap] check callsign count error: %v", err)
		return false
	}

	if phoneCount > 0 {
		n, err := rand.Int(rand.Reader, big.NewInt(1000000000))
		if err != nil {
			log.Printf("[bootstrap] generate phone failed: %v", err)
			return false
		}
		phone = fmt.Sprintf("10%09d", n.Int64())
	}
	if callsignCount > 0 {
		n, err := rand.Int(rand.Reader, big.NewInt(10000))
		if err != nil {
			log.Printf("[bootstrap] generate callsign suffix failed: %v", err)
			return false
		}
		callsign = fmt.Sprintf("%s-%04d", callsign, n.Int64())
	}

	res, err := db.Exec(`INSERT INTO users
		(name, callsign, phone, password, roles, status, alarm_msg,
		 routes, must_change_pwd, create_time, update_time, login_err_times)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)`,
		callsign+" Admin",
		callsign,
		phone,
		string(hash),
		"admin",
		1,
		0,
		"",
		0,
	)
	if err != nil {
		log.Printf("[bootstrap] seed admin user failed: %v", err)
		return false
	}

	if id, err := res.LastInsertId(); err != nil {
		log.Printf("[bootstrap] get seeded admin id failed: %v", err)
	} else {
		setMeta("default_admin_id", strconv.Itoa(int(id)))
	}

	separator := strings.Repeat("=", 60)
	fmt.Println(separator)
	fmt.Println("  首次启动 — 默认管理员账户已创建")
	fmt.Println()
	fmt.Printf("  呼号 (Callsign): %s\n", callsign)
	fmt.Printf("  密码 (Password):  %s\n", password)
	fmt.Println()
	fmt.Println("  首次登录后请立即修改密码！")
	fmt.Println(separator)

	addOperatorLog("默认管理员已创建", "系统初始化", &userinfo{})

	log.Printf("[bootstrap] 默认管理员创建完成，callsign=%s，密码已输出到 stdout", callsign)
	return true
}

func validatePassword(pw string) error {
	if len(pw) < 8 {
		return fmt.Errorf("密码长度不能少于8位")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range pw {
		switch {
		case ch >= 'A' && ch <= 'Z':
			hasUpper = true
		case ch >= 'a' && ch <= 'z':
			hasLower = true
		case ch >= '0' && ch <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("密码必须包含至少一个大写字母")
	}
	if !hasLower {
		return fmt.Errorf("密码必须包含至少一个小写字母")
	}
	if !hasDigit {
		return fmt.Errorf("密码必须包含至少一个数字")
	}
	if !hasSpecial {
		return fmt.Errorf("密码必须包含至少一个特殊字符")
	}

	blacklist := []string{"nrl1234", "nrl888", "admin123", "password", "12345678"}
	for _, b := range blacklist {
		if strings.Contains(strings.ToLower(pw), b) {
			return fmt.Errorf("密码包含不安全词汇")
		}
	}

	return nil
}

func clearMustChangePwd(userID int) {
	_, err := db.Exec("UPDATE users SET must_change_pwd=0 WHERE id=?", userID)
	if err != nil {
		log.Printf("[bootstrap] clear must_change_pwd flag for user %d error: %v", userID, err)
	}
}
