package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	//"github.com/lib/pq"
)

type userinfo struct {
	//uUID     string `db:"uuid"`
	PID      string `db:"pid" json:"pid"`
	ID       int    `db:"id" json:"id"`
	Name     string `db:"name" json:"name"`
	CallSign string `db:"callsign" json:"callsign"`
	MDCID    string `db:"mdcid" json:"mdcid"`
	DMRID    string `db:"dmrid" json:"dmrid"`
	Gird     string `db:"gird" json:"gird"`
	Phone    string `db:"phone" json:"phone"`
	//	JobTime  string `db:"job_time" json:"job_time"`
	Birthday string `db:"birthday" json:"birthday"`
	Sex      int    `db:"sex" json:"sex"`
	Address  string
	Mail     string
	//CanSpeekerDev *connPoll
	//GroupsList []map[uint64]bool
	DevList map[int]*deviceInfo `json:"devlist"` //key 房间号
	//ConnPoll map[int]*currentConnPool //群组连接池表，每个组有一个连接池列表 /key为组号
	Groups map[int]*group //呼号map
	//	userID        int            `db:"user_id" json:"user_id"`
	//Position          int                 `db:"position" json:"position"`
	Introduction string   `db:"introduction" json:"introduction"`
	Avatar       string   `db:"avatar" json:"avatar"`
	Roles        []string `db:"roles" json:"roles"`
	UpdateTime   string   `db:"update_time" json:"update_time"`
	CreateTime   string   `db:"create_time" json:"create_time"`

	Routes        string        `json:"routes" db:"routes"`
	Status        int           `json:"status" db:"status"`
	LastLoginTime string        `json:"last_login_time" db:"last_login_time"`
	LastLoginIP   string        `json:"last_login_ip" db:"last_login_ip"`
	ExpireTime    string        `json:"expire_time" db:"expire_time"`
	LoginErrTimes int           `json:"login_err_times" db:"login_err_times"`
	AlarmMsg      bool          `json:"alarm_msg" db:"alarm_msg"`
	NickName      string        `json:"nickname" db:"nickname"`
	OpenID        string        `json:"openid" db:"openid"`
	MustChangePwd int           `json:"must_change_pwd" db:"must_change_pwd"`
	LockUntil     int64         `json:"lock_until" db:"lock_until"`
	TokenVersion  int           `json:"token_version" db:"token_version"`
	TOTPSecret    string        `json:"-" db:"totp_secret"`
	TOTPEnabled   int           `json:"totp_enabled" db:"totp_enabled"`
	TalkDuration  time.Duration `json:"talk_duration"`
	TalkTimes     int           `json:"talk_times"`
}

type role struct {
	ID          int    `json:"id" db:"id"`
	NameKey     string `json:"key" db:"name_key"`
	Name        string `json:"name" db:"name"`
	Description string `json:"description" db:"description"`
	Routes      string `json:"routes" db:"routes"`
}

// func getRoutes() []role {
// 	r := []role{}

// 	err := db.Select(&r, "SELECT * FROM routes ")

// 	if err != nil {
// 		log.Println("查询菜单路由表错误: ", err)
// 		return nil

// 	}
// 	//fmt.Println(emp)
// 	return r

// }

func getRoles(query string) []*role {
	rl := []*role{}

	q := fmt.Sprintf("SELECT id, name_key, name, description, COALESCE(routes, '') FROM roles %v ", query)

	rows, err := db.Query(q)

	if err != nil {
		log.Println("查询角色列表错误: ", err)
		return nil

	}
	defer rows.Close()

	for rows.Next() {
		r := &role{}

		err = rows.Scan(&r.ID, &r.NameKey, &r.Name, &r.Description, &r.Routes)

		if err != nil {
			log.Println("select roles err :", err, query)
			continue
		}

		rl = append(rl, r)

	}
	//fmt.Println(emp)
	return rl

}

func (s *role) String() string {

	return fmt.Sprintf("NameKey:%v Name:%v", s.NameKey, s.Name)

}

func (s *userinfo) String() string {

	return fmt.Sprintf("Name:%v Phone:%v ", s.Name, s.Phone)

}

func getRoleByKey(key string) *role {

	r := &role{}

	row := db.QueryRow("SELECT id, name_key, name, description, COALESCE(routes, '') from roles where name_key=?", key)
	err := row.Scan(&r.ID, &r.NameKey, &r.Name, &r.Description, &r.Routes)
	if err != nil {
		log.Println("query role by key err:", err, r, key)
	}
	return r
}

func addRole(r *role) error {
	res, err := db.Exec("INSERT INTO roles (name_key,name,description,routes) VALUES (?,?,?,?)", r.NameKey, r.Name, r.Description, r.Routes)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	r.ID = int(id)
	return err
}

func updateRole(r *role) error {
	res, err := db.Exec("UPDATE roles SET name_key=?,name=?,description=?,routes=? WHERE id=?", r.NameKey, r.Name, r.Description, r.Routes, r.ID)
	return roleWriteResult(res, err)
}

func delRole(key string) error {
	if key == "admin" {
		return errors.New("不能删除内置管理员角色")
	}
	res, err := db.Exec("DELETE FROM roles WHERE name_key=?", key)
	return roleWriteResult(res, err)
}

func roleWriteResult(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count == 0 {
		return errors.New("指定的角色不存在")
	}
	return err
}

func selectuser(w string, args []interface{}, p string, sort string) ([]userinfo, int) {

	emp := []userinfo{}

	query := fmt.Sprintf(`SELECT id,pid,name,phone,
	 callsign,gird,birthday,mdcid,dmrid,
	 sex,nickname,openid,avatar,address, status,
	 last_login_time, login_err_times, last_login_ip,
	 alarm_msg,roles,create_time,update_time,expire_time,
	 COALESCE(lock_until, 0), COALESCE(token_version, 1) FROM users  %v  %v  %v  `, w, sort, p)

	//fmt.Println(query)

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Println("查询用户列表错误: ", err, "\n", query)
		return nil, 0
	}
	defer rows.Close()

	for rows.Next() {

		r := userinfo{}
		var roles string
		err := rows.Scan(&r.ID, &r.PID, &r.Name, &r.Phone,
			&r.CallSign, &r.Gird, &r.Birthday, &r.MDCID, &r.DMRID,
			&r.Sex, &r.NickName, &r.OpenID, &r.Avatar, &r.Address, &r.Status,
			&r.LastLoginTime, &r.LoginErrTimes, &r.LastLoginIP,
			&r.AlarmMsg, &roles, &r.CreateTime, &r.UpdateTime, &r.ExpireTime,
			&r.LockUntil, &r.TokenVersion,
		)
		if err != nil {
			log.Println("getuser by username err :", err, "\n", query)
			continue
		}
		r.Roles = strings.Split(roles, ",")
		r.Phone = decryptField(fieldAESKey, r.Phone)
		emp = append(emp, r)
	}

	var t int
	q := fmt.Sprintf(`SELECT count(*) as total FROM users  %v  `, w)
	//fmt.Println(q)
	row := db.QueryRow(q, args...)
	err = row.Scan(&t)
	if err != nil {
		log.Println(" 查询用户列表total错误 err:", err, t)
		return nil, 0
	}
	//fmt.Println(emp)
	return emp, t
	//fmt.Println(emp)

}

// 初始化和旧版本创建的账号可能缺少选填字段，统一按空值读取。
const userRecordSelect = `SELECT id, COALESCE(pid,''), COALESCE(name,''), COALESCE(phone,''),
	COALESCE(callsign,''), COALESCE(gird,''), COALESCE(birthday,''), COALESCE(mdcid,''), COALESCE(dmrid,''),
	COALESCE(sex,0), COALESCE(nickname,''), COALESCE(openid,''), COALESCE(avatar,''), COALESCE(address,''), COALESCE(status,0),
	COALESCE(last_login_time,''), COALESCE(login_err_times,0), COALESCE(last_login_ip,''),
	COALESCE(alarm_msg,0), COALESCE(roles,''), COALESCE(create_time,''), COALESCE(update_time,''), COALESCE(expire_time,''),
	COALESCE(must_change_pwd,0), COALESCE(lock_until,0), COALESCE(token_version,1),
	COALESCE(totp_secret,''), COALESCE(totp_enabled,0), COALESCE(introduction,''), COALESCE(routes,'') FROM users`

func getuser(username string) (*userinfo, error) {
	username = strings.ToUpper(username)
	phoneHash := hashIndex(fieldPepperKey, username)
	return readUserRecord(db.QueryRow(userRecordSelect+" WHERE phone_hash=? OR phone=? OR callsign=?", phoneHash, username, username))
}

func getuserByID(id int) (*userinfo, error) {
	return readUserRecord(db.QueryRow(userRecordSelect+" WHERE id=?", id))
}

func readUserRecord(row *sql.Row) (*userinfo, error) {
	r := &userinfo{}
	var roles string
	err := row.Scan(&r.ID, &r.PID, &r.Name, &r.Phone,
		&r.CallSign, &r.Gird, &r.Birthday, &r.MDCID, &r.DMRID,
		&r.Sex, &r.NickName, &r.OpenID, &r.Avatar, &r.Address, &r.Status,
		&r.LastLoginTime, &r.LoginErrTimes, &r.LastLoginIP, &r.AlarmMsg, &roles,
		&r.CreateTime, &r.UpdateTime, &r.ExpireTime, &r.MustChangePwd,
		&r.LockUntil, &r.TokenVersion, &r.TOTPSecret, &r.TOTPEnabled, &r.Introduction, &r.Routes)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Println("read user record:", err)
		}
		return nil, err
	}
	r.Roles = strings.Split(roles, ",")
	r.Phone = decryptField(fieldAESKey, r.Phone)
	return r, nil
}

// cacheUserIndexes 维护用户内存双索引：userlist 按 CallSign，userPhoneIndex 按 Phone。
// 只存经过 userinit 的用户对象（Groups 等字段被其它模块依赖）。
func cacheUserIndexes(u *userinfo) {
	if u.CallSign == "" {
		return
	}
	userlist.Store(u.CallSign, u)
	if u.Phone != "" {
		userPhoneIndex.Store(u.Phone, u)
	}
}

func uncacheUserIndexes(u *userinfo) {
	if u.CallSign != "" {
		userlist.Delete(u.CallSign)
	}
	if u.Phone != "" {
		userPhoneIndex.Delete(u.Phone)
	}
}

// getUserCached 优先从内存索引取用户（checktoken 每请求调用的热路径），未命中回源 DB。
// 命中缓存时返回共享指针，调用方按只读使用；DB 回源返回全新副本（不回填索引，
// 因为 getuser 的结果未做 userinit，直接入 userlist 会缺 Groups 等字段）。
func getUserCached(username string) (*userinfo, error) {
	username = strings.ToUpper(username)

	if v, ok := userlist.Load(username); ok {
		return v.(*userinfo), nil
	}
	if v, ok := userPhoneIndex.Load(username); ok {
		return v.(*userinfo), nil
	}
	return getuser(username)
}

func getEmpListByRole(role string) ([]userinfo, int) {

	emp := []userinfo{}

	// 不查询 password（bcrypt 哈希不应随接口返回）
	query := `SELECT id,name,callsign,gird,phone,birthday,mdcid,dmrid,
	sex,avatar,address,roles,introduction,alarm_msg,status,update_time,last_login_time,
	login_err_times,create_time,openid,nickname,pid,last_login_ip,expire_time FROM users
	 where  roles like ?  ORDER BY id ASC`

	rows, err := db.Query(query, "%"+role+"%")

	if err != nil {
		log.Println("按角色查询用户列表错误: ", err, '\n', query)
		return nil, 0

	}
	defer rows.Close()

	for rows.Next() {

		r := userinfo{}
		var roles string
		err := rows.Scan(&r.ID, &r.Name, &r.CallSign, &r.Gird, &r.Phone, &r.Birthday, &r.MDCID, &r.DMRID,
			&r.Sex, &r.Avatar, &r.Address,
			&roles, &r.Introduction, &r.AlarmMsg, &r.Status, &r.UpdateTime, &r.LastLoginTime, &r.LoginErrTimes,
			&r.CreateTime, &r.OpenID, &r.NickName, &r.PID, &r.LastLoginIP, &r.ExpireTime)
		if err != nil {
			log.Println("getuser by username err :", err, "\n", query)
			continue
		}
		r.Roles = strings.Split(roles, ",")
		emp = append(emp, r)
	}

	var t int
	q := `SELECT count(*) as total FROM users where  roles like ? `
	row := db.QueryRow(q, "%"+role+"%")
	err = row.Scan(&t)

	if err != nil {
		log.Println(" 查询教师用户列表total错误 err:", err, '\n', q, t)
		return nil, 0
	}
	//fmt.Println(emp)
	return emp, t
	//fmt.Println(emp)

}

func checkrole(emp *userinfo, roles []string) bool {

	//db.Exec(`select  '{"admin"}' <@ roles from user where ` )
	for _, rv := range roles {
		for _, v := range emp.Roles {
			if v == rv {
				return true
			}
		}
	}

	return false
}

func loginCheck(password string, username string, ip string) ([]string, bool) {

	//pass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	//fmt.Println("user login:", username, password, string(pass), err)

	username = strings.ToUpper(username)

	type resault struct {
		Password      string   `db:"password"`
		Roles         []string `db:"roles"`
		Status        int      `db:"status"`
		LoginErrTimes int      `db:"login_err_times"`
		LockUntil     int64    `db:"lock_until"`
	}
	r := &resault{}

	var roles string

	phoneHash := hashIndex(fieldPepperKey, username)
	row := db.QueryRow("SELECT password, login_err_times, status, roles, COALESCE(lock_until, 0) FROM users where phone_hash=? or phone=? or callsign=?", phoneHash, username, username)
	err := row.Scan(&r.Password, &r.LoginErrTimes, &r.Status, &roles, &r.LockUntil)
	if err != nil {
		// 不打印 password：登录失败路径不能把明文密码写进日志
		log.Println("login err:", err, "username:", username)
		return nil, false
	}

	r.Roles = strings.Split(roles, ",")

	now := time.Now().Unix()
	// 如果仍在锁定时间内，直接拒绝
	if r.LockUntil > now {
		return nil, false
	}

	// 如果锁定窗口已过，自动解除锁定并重置计数
	if r.LockUntil > 0 && r.LockUntil <= now {
		r.LoginErrTimes = 0
		r.LockUntil = 0
		_, _ = db.Exec("update users set login_err_times=0, lock_until=0 where phone_hash=? or phone=? or callsign=?", phoneHash, username, username)
	}

	// 账号已锁定时直接拒绝，避免每次请求都执行 bcrypt（CPU DoS 放大器）
	if r.LoginErrTimes >= conf.Security.MaxLoginAttempts {
		return nil, false
	}

	var passwordOK bool

	err = bcrypt.CompareHashAndPassword([]byte(r.Password), []byte(password))
	if err == nil {
		passwordOK = true
	}

	if passwordOK {
		_, err = db.Exec(`update users set last_login_time=CURRENT_TIMESTAMP,last_login_ip=?,login_err_times=0,lock_until=0 where phone_hash=? or phone=? or callsign=?`, ip, phoneHash, username, username)
		if err != nil {
			log.Println("update users last_login_time and last_login_ip  failed, ", err)
			return nil, false
		}

	}

	if !passwordOK {
		newErrTimes := r.LoginErrTimes + 1
		var lockUntil int64 = 0
		if newErrTimes >= conf.Security.MaxLoginAttempts {
			duration := conf.Security.LockoutDuration
			if duration <= 0 {
				duration = 900
			}
			lockUntil = now + int64(duration)
		}
		_, err = db.Exec(`update users set login_err_times=?, lock_until=? where phone_hash=? or phone=? or callsign=?`, newErrTimes, lockUntil, phoneHash, username, username)
		if err != nil {
			log.Println("update user login_err_times failed ,and password err ", err)
			return nil, false
		}

	}

	//fmt.Println(r.PasswordOK, r.Status, r.LoginErrTimes)

	return r.Roles, passwordOK && r.Status == 1 && r.LoginErrTimes < conf.Security.MaxLoginAttempts

}

// func roleCheck() {
// 	db.Exec(`select  '{"admin"}' <@ role from user`)
// }

// func updateop() {
// 	res, err := db.Exec("update user set username=? where user_id=?", "stu0003", 1)
// 	if err != nil {
// 		fmt.Println("exec failed, ", err)
// 		return
// 	}
// 	row, err := res.RowsAffected()
// 	if err != nil {
// 		fmt.Println("rows failed, ", err)
// 	}
// 	fmt.Println("update succ:", row)
// }

// func updatePassword() {
// 	db.Exec("UPDATE user  SET password = crypt('123', gen_salt('md5'))")
// 	db.Exec("SELECT password = crypt('123', password) FROM user where mobile_phone=")
// }

func addUser(e *userinfo, newPassword string) error {

	if err := validatePassword(newPassword, e.CallSign, e.Phone); err != nil {
		return err
	}

	password, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)

	if err != nil {
		return err
	}

	// 插入数据

	//	fmt.Println("user:", e)

	phoneToStore := e.Phone
	if fieldAESKey != nil {
		if enc, err := encryptField(fieldAESKey, e.Phone); err == nil {
			phoneToStore = enc
		}
	}
	phoneHash := hashIndex(fieldPepperKey, e.Phone)

	roles := strings.Join(e.Roles, ",")
	query := `INSERT INTO users (pid,name,phone,sex,callsign,mdcid,dmrid,gird,address,birthday,introduction,nickname,openid,last_login_ip,last_login_time,
		avatar,status,password,roles, alarm_msg,		
		expire_time,create_time,login_err_times,update_time,phone_hash) 
	VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,
		CURRENT_TIMESTAMP,0,CURRENT_TIMESTAMP,?)`

	stmt, err := db.Prepare(query)

	if err != nil {
		log.Println("add user failed 1 , ", err, '\n', query)
		return err
	}
	defer stmt.Close()

	//	e.Avatar = "https://wpimg.wallstcn.com/f778738c-e4f8-4870-b634-56703b4acafe.gif"
	e.Avatar = "" // 默认头像留空，由前端展示占位图

	res, err := stmt.Exec(e.PID, e.Name, phoneToStore, e.Sex, e.CallSign, e.MDCID, e.DMRID, e.Gird, e.Address, e.Birthday, e.Introduction, e.NickName, e.OpenID, e.LastLoginIP, e.LastLoginTime,
		e.Avatar, e.Status, password, roles, e.AlarmMsg, e.ExpireTime, phoneHash)
	// Named queries can use structs, so if you have an existing struct (i.e. person := &Person{}) that you have populated, you can pass it in as &person
	//	tx.NamedExec("INSERT INTO person (first_name, last_name, email) VALUES (:first_name, :last_name, :email)", &Person{"Jane", "Citizen", "jane.citzen@example.com"})
	if err != nil {
		log.Println("add user failed 2, ", err, '\n', query)
		return err
	}

	id, err := res.LastInsertId()

	if err != nil {
		log.Println("add user failed 3, ", err, '\n', query)
		return err
	}

	return reloadUserIndexes(int(id), nil)
}

func deleteUser(e *userinfo) error {

	_, err := db.Exec("delete from users where id=?", e.ID)
	if err != nil {
		log.Println("delete user failed, ", err)
		return err
	}

	// 先从内存缓存取出旧值，再清理非空的 mdcid/dmrid 映射（参照 updateUserProfile 的做法）
	oldPhone := e.Phone
	if oldUser, ok := userlist.Load(e.CallSign); ok {
		old := oldUser.(*userinfo)
		oldPhone = old.Phone
		if old.MDCID != "" {
			mdcidmap.Delete(old.MDCID)
		}
		if old.DMRID != "" {
			dmridmap.Delete(old.DMRID)
		}
	}

	userlist.Delete(e.CallSign)
	if oldPhone != "" {
		userPhoneIndex.Delete(oldPhone)
	}

	return nil
}

func updateUser(e *userinfo, password string) error {
	phoneToStore := e.Phone
	if fieldAESKey != nil {
		var err error
		phoneToStore, err = encryptField(fieldAESKey, e.Phone)
		if err != nil {
			return err
		}
	}
	fields := `name=?,phone=?,sex=?,callsign=?,mdcid=?,dmrid=?,gird=?,address=?,birthday=?,introduction=?,
		avatar=?,status=?,alarm_msg=?,expire_time=?,roles=?,pid=?,phone_hash=?,
		login_err_times=CASE WHEN ?=1 THEN 0 ELSE login_err_times END`
	args := []any{e.Name, phoneToStore, e.Sex, e.CallSign, e.MDCID, e.DMRID, e.Gird, e.Address, e.Birthday,
		e.Introduction, e.Avatar, e.Status, e.AlarmMsg, e.ExpireTime, strings.Join(e.Roles, ","), e.PID,
		hashIndex(fieldPepperKey, e.Phone), e.Status}
	return saveUserFields(e.ID, fields, args, password, e.CallSign, e.Phone)
}

func updateUseravatar(e *userinfo) error {

	_, err := db.Exec(`update users set avatar=?,   update_time=CURRENT_TIMESTAMP  where id=?`,
		e.Avatar, e.ID)
	if err != nil {
		log.Println("update user failed, ", err)
		return err
	}

	if u, okok := userlist.Load(e.CallSign); okok {
		u.(*userinfo).Avatar = e.Avatar

	}

	return nil

}

func updateUserProfile(u *userinfo, dmrid, mdcid, avatar, password string) error {
	return saveUserFields(u.ID, "dmrid=?, mdcid=?, avatar=?", []any{dmrid, mdcid, avatar}, password, u.CallSign, u.Phone)
}

// 串行发布账号写入和缓存刷新，避免后完成的请求把旧令牌版本写回缓存。
var userWriteMu sync.Mutex

// fields 只接受调用方写死的列赋值，所有数据用 SQL 参数传入。
func saveUserFields(id int, fields string, args []any, password, callsign, phone string) error {
	query := "UPDATE users SET update_time=CURRENT_TIMESTAMP"
	if fields != "" {
		query += ", " + fields
	}
	if password != "" {
		if err := validatePassword(password, callsign, phone); err != nil {
			return err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		query += ", password=?, must_change_pwd=0, token_version=COALESCE(token_version, 1)+1"
		args = append(args, hash)
	}
	userWriteMu.Lock()
	defer userWriteMu.Unlock()
	current, err := getuserByID(id)
	if err != nil {
		return err
	}
	if _, err := db.Exec(query+" WHERE id=?", append(args, id)...); err != nil {
		return err
	}
	return reloadUserIndexes(current.ID, current)
}

func reloadUserIndexes(id int, old *userinfo) error {
	u, err := getuserByID(id)
	if err != nil {
		return err
	}
	cached, cachedOK := userlist.Load(u.CallSign)
	if old != nil {
		if old.CallSign != u.CallSign || old.Phone != u.Phone {
			uncacheUserIndexes(old)
		}
		if old.MDCID != "" && old.MDCID != u.MDCID {
			mdcidmap.Delete(old.MDCID)
		}
		if old.DMRID != "" && old.DMRID != u.DMRID {
			dmridmap.Delete(old.DMRID)
		}
	}
	if cachedOK && cached.(*userinfo).ID == u.ID {
		previous := cached.(*userinfo)
		u.Groups, u.DevList = previous.Groups, previous.DevList
		u.TalkDuration, u.TalkTimes = previous.TalkDuration, previous.TalkTimes
	} else {
		u.userinit()
	}
	cacheUserIndexes(u)
	if u.MDCID != "" {
		mdcidmap.Store(u.MDCID, u.CallSign)
	}
	if u.DMRID != "" {
		dmridmap.Store(u.DMRID, u.CallSign)
	}
	return nil
}

func revokeUserToken(id int) error {
	userWriteMu.Lock()
	defer userWriteMu.Unlock()
	if _, err := db.Exec("UPDATE users SET token_version = COALESCE(token_version, 1) + 1 WHERE id=?", id); err != nil {
		return err
	}
	return reloadUserIndexes(id, nil)
}
