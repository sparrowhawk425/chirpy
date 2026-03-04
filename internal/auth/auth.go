package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	return argon2id.CreateHash(password, argon2id.DefaultParams)
}

func CheckPasswordHash(password, hash string) (bool, error) {
	return argon2id.ComparePasswordAndHash(password, hash)
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject:   userID.String(),
	})
	tokenString, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
	token, err := jwt.ParseWithClaims(tokenString, jwt.MapClaims{},
		func(t *jwt.Token) (any, error) {
			return []byte(tokenSecret), nil
		})
	if err != nil {
		return uuid.UUID{}, err
	}
	subject, err := token.Claims.GetSubject()
	if err != nil {
		return uuid.UUID{}, err
	}
	userId, err := uuid.Parse(subject)
	if err != nil {
		return uuid.UUID{}, err
	}
	return userId, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	return parseToken(headers, "Bearer")
}

func MakeRefreshToken() string {
	token := make([]byte, 32)
	rand.Read(token)
	return hex.EncodeToString(token)
}

func GetAPIKey(headers http.Header) (string, error) {
	return parseToken(headers, "ApiKey")
}

func parseToken(headers http.Header, prefix string) (string, error) {
	token := headers.Get("Authorization")
	if len(token) == 0 {
		return "", fmt.Errorf("No Authorization header found")
	}
	fields := strings.Fields(token)
	if len(fields) != 2 || !strings.EqualFold(fields[0], prefix) {
		return "", fmt.Errorf("Token has unexpected format")
	}
	return fields[1], nil
}
