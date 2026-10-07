package helper

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"siakad-mini/app/model"
)

var (
	ErrTokenExpired = errors.New("token kedaluwarsa")
	ErrTokenInvalid = errors.New("token tidak valid")
)

// accessClaims adalah isi (payload) dari access token.
type accessClaims struct {
	UserID int    `json:"uid"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// JWTManager bertugas membuat dan memeriksa token.
type JWTManager struct {
	secret    []byte
	issuer    string
	accessTTL time.Duration
}

func NewJWTManager(secret, issuer string, accessTTL time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), issuer: issuer, accessTTL: accessTTL}
}

// AccessTTL mengembalikan masa berlaku token.
func (m *JWTManager) AccessTTL() time.Duration { return m.accessTTL }

// GenerateAccess membuat access token untuk user yang berhasil login.
func (m *JWTManager) GenerateAccess(u model.User) (string, error) {
	now := time.Now()
	claims := accessClaims{
		UserID: u.ID,
		Email:  u.Email,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprint(u.ID),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

// Parse memeriksa token dan mengembalikan identitas pemiliknya.
func (m *JWTManager) Parse(tokenString string) (model.AuthUser, error) {
	claims := &accessClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims,
		func(t *jwt.Token) (any, error) {
			// Tolak token yang memakai algoritma selain HMAC.
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, ErrTokenInvalid
			}
			return m.secret, nil
		},
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return model.AuthUser{}, ErrTokenExpired
		}
		return model.AuthUser{}, ErrTokenInvalid
	}
	return model.AuthUser{ID: claims.UserID, Email: claims.Email, Role: claims.Role}, nil
}
