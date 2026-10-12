package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

type waUser struct {
	user  *userinfo
	creds []webauthn.Credential
}

func (u *waUser) WebAuthnID() []byte {
	return []byte(strconv.Itoa(u.user.ID))
}

func (u *waUser) WebAuthnName() string {
	return u.user.CallSign
}

func (u *waUser) WebAuthnDisplayName() string {
	if u.user.Name != "" {
		return u.user.Name
	}
	return u.user.CallSign
}

func (u *waUser) WebAuthnCredentials() []webauthn.Credential {
	return u.creds
}

func getWebAuthn(req *http.Request) (*webauthn.WebAuthn, error) {
	host := req.Host
	rpid := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		rpid = h
	}

	proto := "http"
	if req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https" {
		proto = "https"
	}
	origin := proto + "://" + host

	origins := []string{origin}
	if !strings.Contains(host, "://") {
		origins = append(origins, "http://"+host, "https://"+host)
	}

	return webauthn.New(&webauthn.Config{
		RPDisplayName: "NRLLink",
		RPID:          rpid,
		RPOrigins:     origins,
	})
}

func loadUserCredentials(userID int) ([]webauthn.Credential, error) {
	rows, err := db.Query(`SELECT credential_id, public_key, COALESCE(attestation_type, ''), COALESCE(aaguid, ''), COALESCE(sign_count, 0)
		FROM user_credentials WHERE user_id=?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []webauthn.Credential
	for rows.Next() {
		var credID, pubKey, aaguid []byte
		var attType string
		var signCount uint32
		if err := rows.Scan(&credID, &pubKey, &attType, &aaguid, &signCount); err != nil {
			continue
		}
		creds = append(creds, webauthn.Credential{
			ID:              credID,
			PublicKey:       pubKey,
			AttestationType: attType,
			Authenticator: webauthn.Authenticator{
				AAGUID:    aaguid,
				SignCount: signCount,
			},
		})
	}
	return creds, nil
}

func saveUserCredential(userID int, cred *webauthn.Credential, name string) error {
	if name == "" {
		name = "通行密钥 " + time.Now().Format("01-02 15:04")
	}
	_, err := db.Exec(`INSERT INTO user_credentials
		(user_id, credential_id, public_key, attestation_type, aaguid, sign_count, name, create_time, last_used_time)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		userID, cred.ID, cred.PublicKey, cred.AttestationType, cred.Authenticator.AAGUID, cred.Authenticator.SignCount, name)
	return err
}

func updateUserCredentialUsage(credID []byte, newCount uint32) {
	_, _ = db.Exec(`UPDATE user_credentials SET sign_count=?, last_used_time=CURRENT_TIMESTAMP WHERE credential_id=?`, newCount, credID)
}

type waSessionRecord struct {
	sessionData webauthn.SessionData
	userID      int
	expireAt    time.Time
}

var waSessionStore sync.Map

func genSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// httpWebAuthnRegisterBegin 开始通行密钥注册仪式
func (j *jsonapi) httpWebAuthnRegisterBegin(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	wa, err := getWebAuthn(req)
	if err != nil {
		log.Println("getWebAuthn err:", err)
		writeJSONResponseError(w, "初始化通行密钥引擎失败: "+err.Error())
		return
	}

	creds, _ := loadUserCredentials(u.ID)
	user := &waUser{user: u, creds: creds}

	options, sessionData, err := wa.BeginRegistration(user,
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{
			UserVerification: protocol.VerificationPreferred,
		}),
	)
	if err != nil {
		log.Println("BeginRegistration err:", err)
		writeJSONResponseError(w, "生成注册凭据失败")
		return
	}

	sessionID := genSessionID()
	waSessionStore.Store(sessionID, waSessionRecord{
		sessionData: *sessionData,
		userID:      u.ID,
		expireAt:    time.Now().Add(5 * time.Minute),
	})

	writeJSONEnvelope(w, 20000, map[string]any{
		"session_id": sessionID,
		"options":    options,
	})
}

// httpWebAuthnRegisterFinish 完成通行密钥注册仪式并入库
func (j *jsonapi) httpWebAuthnRegisterFinish(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	sessionID := req.URL.Query().Get("session_id")
	val, ok := waSessionStore.LoadAndDelete(sessionID)
	if !ok {
		writeJSONResponseMessage(w, "注册会话已过期，请重新发起", 20001)
		return
	}
	record := val.(waSessionRecord)
	if time.Now().After(record.expireAt) || record.userID != u.ID {
		writeJSONResponseMessage(w, "注册会话已过期或用户不匹配", 20001)
		return
	}

	wa, err := getWebAuthn(req)
	if err != nil {
		writeJSONResponseError(w, "校验引擎失败")
		return
	}

	creds, _ := loadUserCredentials(u.ID)
	user := &waUser{user: u, creds: creds}

	credential, err := wa.FinishRegistration(user, record.sessionData, req)
	if err != nil {
		log.Println("FinishRegistration err:", err)
		writeJSONResponseMessage(w, "通行密钥校验失败: "+err.Error(), 20001)
		return
	}

	credName := req.URL.Query().Get("name")
	if err := saveUserCredential(u.ID, credential, credName); err != nil {
		log.Println("saveUserCredential err:", err)
		writeJSONResponseError(w, "保存通行密钥失败")
		return
	}

	addOperatorLog("注册通行密钥 (Passkey)", "安全设置", u)
	writeJSONResponseOK(w)
}

// httpWebAuthnLoginBegin 开始通行密钥免密登录认证
func (j *jsonapi) httpWebAuthnLoginBegin(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	username := strings.TrimSpace(req.URL.Query().Get("username"))

	wa, err := getWebAuthn(req)
	if err != nil {
		writeJSONResponseError(w, "初始化认证引擎失败")
		return
	}

	var options *protocol.CredentialAssertion
	var sessionData *webauthn.SessionData
	var targetUserID int

	if username != "" {
		u, err := getUserCached(username)
		if err != nil {
			writeJSONResponseMessage(w, "用户不存在", 20001)
			return
		}
		creds, _ := loadUserCredentials(u.ID)
		if len(creds) == 0 {
			writeJSONResponseMessage(w, "该用户尚未绑定通行密钥", 20001)
			return
		}
		targetUserID = u.ID
		user := &waUser{user: u, creds: creds}
		options, sessionData, err = wa.BeginLogin(user,
			webauthn.WithUserVerification(protocol.VerificationPreferred),
		)
	} else {
		// Discoverable Credential (一键选择凭据登录)
		options, sessionData, err = wa.BeginDiscoverableLogin(
			webauthn.WithUserVerification(protocol.VerificationPreferred),
		)
	}

	if err != nil {
		log.Println("BeginLogin err:", err)
		writeJSONResponseError(w, "生成登录挑战失败: "+err.Error())
		return
	}

	sessionID := genSessionID()
	waSessionStore.Store(sessionID, waSessionRecord{
		sessionData: *sessionData,
		userID:      targetUserID,
		expireAt:    time.Now().Add(5 * time.Minute),
	})

	writeJSONEnvelope(w, 20000, map[string]any{
		"session_id": sessionID,
		"options":    options,
	})
}

// httpWebAuthnLoginFinish 校验通行密钥签名断言并签发 JWT
func (j *jsonapi) httpWebAuthnLoginFinish(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	sessionID := req.URL.Query().Get("session_id")
	val, ok := waSessionStore.LoadAndDelete(sessionID)
	if !ok {
		writeJSONResponseMessage(w, "认证会话已过期，请重新登录", 20001)
		return
	}
	record := val.(waSessionRecord)
	if time.Now().After(record.expireAt) {
		writeJSONResponseMessage(w, "认证会话已过期，请重新登录", 20001)
		return
	}

	wa, err := getWebAuthn(req)
	if err != nil {
		writeJSONResponseError(w, "引擎错误")
		return
	}

	var authenticatedUser *userinfo
	var credential *webauthn.Credential

	if record.userID > 0 {
		u, err := getuserByID(record.userID)
		if err != nil || u.Status != 1 {
			writeJSONResponseMessage(w, "账号不存在或已禁用", 20001)
			return
		}
		creds, _ := loadUserCredentials(u.ID)
		user := &waUser{user: u, creds: creds}
		credential, err = wa.FinishLogin(user, record.sessionData, req)
		if err != nil {
			log.Println("FinishLogin err:", err)
			writeJSONResponseMessage(w, "通行密钥校验失败: "+err.Error(), 20001)
			return
		}
		authenticatedUser = u
	} else {
		// Discoverable 模式：回调查找匹配的凭据与用户
		credentialHandler := func(rawID, userHandle []byte) (webauthn.User, error) {
			userID, err := strconv.Atoi(string(userHandle))
			if err != nil {
				return nil, err
			}
			u, err := getuserByID(userID)
			if err != nil || u.Status != 1 {
				return nil, sql.ErrNoRows
			}
			creds, _ := loadUserCredentials(u.ID)
			authenticatedUser = u
			return &waUser{user: u, creds: creds}, nil
		}

		credential, err = wa.FinishDiscoverableLogin(credentialHandler, record.sessionData, req)
		if err != nil {
			log.Println("FinishDiscoverableLogin err:", err)
			writeJSONResponseMessage(w, "通行密钥校验失败: "+err.Error(), 20001)
			return
		}
	}

	if authenticatedUser == nil || credential == nil {
		writeJSONResponseMessage(w, "登录验证失败", 20001)
		return
	}

	updateUserCredentialUsage(credential.ID, credential.Authenticator.SignCount)

	tokenVersion := authenticatedUser.TokenVersion
	if tokenVersion <= 0 {
		tokenVersion = 1
	}
	tokenString, err := GenerateToken(authenticatedUser.CallSign, authenticatedUser.Roles, tokenVersion)
	if err != nil {
		w.Write(ResTokenErr)
		return
	}

	addOperatorLog("通行密钥免密登录成功 ("+authenticatedUser.CallSign+")", "登录成功", authenticatedUser)

	writeJSONEnvelope(w, 20000, map[string]any{
		"token":           tokenString,
		"must_change_pwd": authenticatedUser.MustChangePwd == 1,
		"default_admin":   isDefaultAdmin(authenticatedUser.ID),
	})
}

// httpWebAuthnListCredentials 列出当前用户已绑定的通行密钥列表
func (j *jsonapi) httpWebAuthnListCredentials(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	rows, err := db.Query(`SELECT id, name, create_time, last_used_time FROM user_credentials WHERE user_id=? ORDER BY id DESC`, u.ID)
	if err != nil {
		writeJSONResponseError(w, "获取通行密钥列表失败")
		return
	}
	defer rows.Close()

	type credItem struct {
		ID           int    `json:"id"`
		Name         string `json:"name"`
		CreateTime   string `json:"create_time"`
		LastUsedTime string `json:"last_used_time"`
	}
	var items []credItem
	for rows.Next() {
		var it credItem
		if err := rows.Scan(&it.ID, &it.Name, &it.CreateTime, &it.LastUsedTime); err == nil {
			items = append(items, it)
		}
	}

	writeJSONResponseItems(w, items, len(items))
}

// httpWebAuthnDeleteCredential 删除指定的通行密钥
func (j *jsonapi) httpWebAuthnDeleteCredential(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	var payload struct {
		ID int `json:"id"`
	}
	if !decodeRequestJSON(w, req, &payload) {
		return
	}

	_, err = db.Exec("DELETE FROM user_credentials WHERE id=? AND user_id=?", payload.ID, u.ID)
	if err != nil {
		writeJSONResponseError(w, "删除通行密钥失败")
		return
	}

	addOperatorLog(fmt.Sprintf("删除通行密钥 ID:%d", payload.ID), "安全设置", u)
	writeJSONResponseOK(w)
}
