// 文件说明：认证基础服务，封装 JWT 签发校验和 bcrypt 密码处理。

package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	ClientPC  = "pc"
	ClientPDA = "pda"
)

type Claims struct {
	UserID  uint64 `json:"uid"`
	Account string `json:"account"`
	Client  string `json:"client"`
	jwt.RegisteredClaims
}

type Service struct {
	secret []byte
	ttl    time.Duration
}

func NewService(secret string, ttl time.Duration) *Service {
	return &Service{secret: []byte(secret), ttl: ttl}
}

// GenerateToken 签发 HS256 JWT，客户端类型会写入声明并参与后续登录隔离。
func (s *Service) GenerateToken(userID uint64, account, client string) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.ttl)
	claims := Claims{
		UserID: userID, Account: account, Client: client,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   fmt.Sprintf("%d", userID),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	return signed, expiresAt, err
}

// ParseToken 只接受 HS256，显式阻断算法混淆攻击。
func (s *Service) ParseToken(raw string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(raw, &Claims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// HashPassword 使用 bcrypt cost 12，兼顾离线暴力破解成本与登录耗时。
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(b), err
}

func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
