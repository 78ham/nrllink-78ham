package main

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// ensureBootstrap 在 updatedb() 之后、initAllUserList() 之前调用。
// 每次启动先清除发行包自带的旧默认管理员账号（purgeLegacyAdmins），
// 再检测是否为首次启动（无用户表或无 admin 角色用户），若是则：
//  1. 创建所有核心表（DDL 覆盖 db/sqlite.sql 全量）
//  2. 创建 admin 角色
//  3. 创建默认 admin 用户（callsign=NOCALL, roles=admin, must_change_pwd=1）
//  4. 打印随机密码到 stdout
func ensureBootstrap() {
	purgeLegacyAdmins()

	if isBootstrapped() {
		return
	}

	log.Println("[bootstrap] 检测到首次启动，正在初始化数据库...")

	execDDL()

	seedRoles()
	seedAdmin()

	log.Println("[bootstrap] 数据库初始化完成")
}

func isBootstrapped() bool {
	var tblCount int
	err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='users'").Scan(&tblCount)
	if err != nil || tblCount == 0 {
		return false
	}

	var adminCount int
	err = db.QueryRow("SELECT count(*) FROM users WHERE roles LIKE '%admin%'").Scan(&adminCount)
	if err != nil {
		return false
	}
	return adminCount > 0
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
			"gird"	TEXT,
			"phone"	TEXT,
			"password"	TEXT,
			"birthday"	TEXT,
			"sex"	BLOB,
			"avatar"	TEXT,
			"address"	TEXT,
			"roles"	TEXT,
			"introduction"	TEXT,
			"alarm_msg"	BLOB,
			"status"	INTEGER,
			"update_time"	TEXT,
			"last_login_time"	TEXT,
			"login_err_times"	INTEGER,
			"create_time"	TEXT,
			"openid"	TEXT,
			"nickname"	TEXT,
			"pid"	TEXT,
			"last_login_ip"	TEXT,
			"expire_time"	TEXT,
			"routes"	TEXT,
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
			"routess"	TEXT,
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
		`CREATE TABLE IF NOT EXISTS "user_reg" (
			"id"	INTEGER UNIQUE,
			"name"	TEXT,
			"callsign"	TEXT,
			"phone"	TEXT,
			"password"	TEXT,
			"image"	BLOB,
			"status"	INTEGER,
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
		_, err := db.Exec("INSERT INTO roles (name_key, name, description) VALUES (?, ?, ?)",
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

func seedAdmin() {
	var count int
	err := db.QueryRow("SELECT count(*) FROM users WHERE roles LIKE '%admin%'").Scan(&count)
	if err != nil {
		log.Printf("[bootstrap] check admin count error: %v", err)
		return
	}
	if count > 0 {
		return
	}

	password, err := randPassword(16)
	if err != nil {
		log.Fatalf("[bootstrap] generate password failed: %v", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("[bootstrap] hash password failed: %v", err)
	}

	_, err = db.Exec(`INSERT INTO users
		(name, callsign, phone, password, roles, status, alarm_msg,
		 routes, create_time, update_time, login_err_times)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?,
			CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)`,
		"NOCALL Admin",
		"NOCALL",
		"00000000000",
		string(hash),
		"admin",
		1,
		0,
		MustChangePwdFlag,
		0,
	)
	if err != nil {
		log.Fatalf("[bootstrap] seed admin user failed: %v", err)
	}

	separator := strings.Repeat("=", 60)
	fmt.Println(separator)
	fmt.Println("  首次启动 — 默认管理员账户已创建")
	fmt.Println()
	fmt.Println("  呼号 (Callsign): NOCALL")
	fmt.Printf("  密码 (Password):  %s\n", password)
	fmt.Println()
	fmt.Println("  首次登录后请立即修改密码！")
	fmt.Println(separator)

	log.Printf("[bootstrap] 默认管理员创建完成，callsign=NOCALL，密码已输出到 stdout")
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

// MustChangePwdFlag 为 init.go 中创建的 admin 用户添加 must_change_pwd 标记。
// 该标记存储在 users 表的 routes 字段中（临时复用），前端检测到此标记时强制跳转改密页。
// 用户改密后清除该标记。
const MustChangePwdFlag = "MUST_CHANGE_PWD"

func clearMustChangePwd(userID int) {
	_, err := db.Exec("UPDATE users SET routes='' WHERE id=? AND routes=?", userID, MustChangePwdFlag)
	if err != nil {
		log.Printf("[bootstrap] clear must_change_pwd flag for user %d error: %v", userID, err)
	}
}
