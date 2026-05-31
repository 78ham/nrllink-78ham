package main

import (
	"crypto/rand"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var TokenKey []byte

// initTokenKey 必须在 conf.init() 之后调用。
// 优先使用配置中的固定密钥(重启不掉线、多实例通用),
// 缺省时回退到 crypto/rand 随机密钥(进程重启会导致已签发 token 失效)。
func initTokenKey() {
	if conf.Web.TokenKey != "" {
		TokenKey = []byte(conf.Web.TokenKey)
		return
	}
	TokenKey = RandString(32)
	log.Println("warning: Web.TokenKey 未配置,本次启动使用随机密钥,服务重启后所有登录令牌将失效,且多实例间令牌不通用")
}

// 定义 payload 的结构
type Claims struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

// 生成 JWT token
func GenerateToken(username string, roles []string) (string, error) {
	// 设置过期时间
	expirationTime := time.Now().Add(24 * 30 * time.Hour)
	claims := &Claims{
		Username: username,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "nrllink",
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	// 创建 token 并签名
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(TokenKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// 验证 JWT token
func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	// 解析和验证 token
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(TokenKey), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("无效的令牌")
	}

	return claims, nil
}

func RandString(n int) []byte {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		log.Fatal("generate random token key failed:", err)
	}
	return bytes
}
