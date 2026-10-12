package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// httpUserTOTPSetup 生成 TOTP 密钥并返回 otpauth URI 与二维码 Base64
func (j *jsonapi) httpUserTOTPSetup(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "NRLLink",
		AccountName: u.CallSign,
		Period:      30,
		Algorithm:   otp.AlgorithmSHA1,
		Digits:      otp.DigitsSix,
	})
	if err != nil {
		log.Println("generate totp key err:", err)
		writeJSONResponseError(w, "生成两步验证密钥失败")
		return
	}

	img, err := key.Image(200, 200)
	if err != nil {
		log.Println("generate totp qr err:", err)
		writeJSONResponseError(w, "生成二维码失败")
		return
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		writeJSONResponseError(w, "编码二维码失败")
		return
	}
	qrBase64 := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	// 临时保存生成的 Secret 到数据库（未启用状态），等待用户验证通过后置 totp_enabled=1
	_, err = db.Exec("UPDATE users SET totp_secret=?, totp_enabled=0 WHERE id=?", key.Secret(), u.ID)
	if err != nil {
		writeJSONResponseError(w, "保存密钥失败")
		return
	}

	writeJSONEnvelope(w, 20000, map[string]any{
		"secret":   key.Secret(),
		"url":      key.URL(),
		"qr_image": qrBase64,
	})
}

// httpUserTOTPVerify 验证用户输入的 6 位动态码，验证成功后正式激活 TOTP
func (j *jsonapi) httpUserTOTPVerify(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	var payload struct {
		Code string `json:"code"`
	}
	if !decodeRequestJSON(w, req, &payload) {
		return
	}

	var secret string
	err = db.QueryRow("SELECT COALESCE(totp_secret, '') FROM users WHERE id=?", u.ID).Scan(&secret)
	if err != nil || secret == "" {
		writeJSONResponseError(w, "尚未生成两步验证密钥，请重新绑定")
		return
	}

	valid := totp.Validate(payload.Code, secret)
	if !valid {
		writeJSONResponseError(w, "动态验证码错误或已过期，请重试")
		return
	}

	_, err = db.Exec("UPDATE users SET totp_enabled=1 WHERE id=?", u.ID)
	if err != nil {
		writeJSONResponseError(w, "激活两步验证失败")
		return
	}

	if cached, ok := userlist.Load(u.CallSign); ok {
		cached.(*userinfo).TOTPEnabled = 1
		cached.(*userinfo).TOTPSecret = secret
	}

	addOperatorLog("启用两步验证 (TOTP)", "安全设置", u)
	writeJSONResponseOK(w)
}

// httpUserTOTPDisable 关闭两步验证
func (j *jsonapi) httpUserTOTPDisable(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	u, err := checktoken(w, req)
	if err != nil {
		return
	}

	var payload struct {
		Password string `json:"password"`
	}
	if !decodeRequestJSON(w, req, &payload) {
		return
	}

	// 关闭 TOTP 需要验证登录密码以保证安全
	target, err := getuserByID(u.ID)
	if err != nil {
		writeJSONResponseError(w, "用户不存在")
		return
	}
	var dbHash string
	_ = db.QueryRow("SELECT password FROM users WHERE id=?", u.ID).Scan(&dbHash)
	if _, ok := loginCheck(payload.Password, target.CallSign, req.RemoteAddr); !ok {
		writeJSONResponseError(w, "密码验证失败，无法关闭两步验证")
		return
	}

	_, err = db.Exec("UPDATE users SET totp_enabled=0, totp_secret='' WHERE id=?", u.ID)
	if err != nil {
		writeJSONResponseError(w, "关闭两步验证失败")
		return
	}

	if cached, ok := userlist.Load(u.CallSign); ok {
		cached.(*userinfo).TOTPEnabled = 0
		cached.(*userinfo).TOTPSecret = ""
	}

	addOperatorLog("关闭两步验证 (TOTP)", "安全设置", u)
	writeJSONResponseOK(w)
}

// 登录阶段二次验证短效凭证存储
type totpLoginTicket struct {
	userID       int
	username     string
	roles        []string
	tokenVersion int
	expireAt     time.Time
}

var totpLoginTicketStore sync.Map

func createTOTPLoginTicket(u *userinfo, roles []string) string {
	ticket := fmt.Sprintf("totp_%x%x", time.Now().UnixNano(), RandString(16))
	totpLoginTicketStore.Store(ticket, totpLoginTicket{
		userID:       u.ID,
		username:     u.CallSign,
		roles:        roles,
		tokenVersion: u.TokenVersion,
		expireAt:     time.Now().Add(3 * time.Minute),
	})
	return ticket
}

// httpAuthTOTPLogin 接收短效 Ticket 与 6 位动态验证码，核对无误后签发最终正式 JWT
func (j *jsonapi) httpAuthTOTPLogin(w http.ResponseWriter, req *http.Request) {
	sethttphead(w)
	var payload struct {
		Ticket string `json:"ticket"`
		Code   string `json:"code"`
	}
	if !decodeRequestJSON(w, req, &payload) {
		return
	}

	val, ok := totpLoginTicketStore.LoadAndDelete(payload.Ticket)
	if !ok {
		writeJSONResponseMessage(w, "登录验证会话已过期，请重新登录", 20001)
		return
	}
	ticket := val.(totpLoginTicket)
	if time.Now().After(ticket.expireAt) {
		writeJSONResponseMessage(w, "登录验证会话已过期，请重新登录", 20001)
		return
	}

	var secret string
	err := db.QueryRow("SELECT COALESCE(totp_secret, '') FROM users WHERE id=?", ticket.userID).Scan(&secret)
	if err != nil || secret == "" {
		writeJSONResponseMessage(w, "用户未绑定两步验证", 20001)
		return
	}

	if !totp.Validate(payload.Code, secret) {
		writeJSONResponseMessage(w, "动态验证码错误，请重新输入", 20001)
		return
	}

	s, err := GenerateToken(ticket.username, ticket.roles, ticket.tokenVersion)
	if err != nil {
		w.Write(ResTokenErr)
		return
	}

	writeJSONEnvelope(w, 20000, map[string]any{
		"token":           s,
		"must_change_pwd": false,
		"default_admin":   isDefaultAdmin(ticket.userID),
	})
}
