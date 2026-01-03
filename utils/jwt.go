package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	UserID string `json:"userId"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func GenerateToken(userId string, userRole string, duration time.Duration) (string, error) {

	JWT_SECRET := os.Getenv("JWT_SECRET")
	if JWT_SECRET == "" {
		return "", errors.New("JWT_SECRET not set in environment")
	}
	claims := JWTClaims{
		UserID: userId,
		Role:   userRole,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "vendora",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(JWT_SECRET))
}

func VerifyToken(tokenString string) (*JWTClaims, error) {
	JWT_SECRET := os.Getenv("JWT_SECRET")
	if JWT_SECRET == "" {
		return nil, errors.New("JWT_SECRET not set in environment")
	}
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(JWT_SECRET), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	return token.Claims.(*JWTClaims), nil
}

// func RefreshToken(tokenString string) (string, error) {

// }

func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func GenerateRefreshToken() (string, error) {
	return GenerateSecureToken(32)
}
