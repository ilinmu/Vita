package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type jwtClaims struct {
	Sub string `json:"sub"`
	Usr string `json:"usr"`
	Exp int64  `json:"exp"`
}

// LoadOrCreateJWTSecret returns secret if non-empty, otherwise reads/creates <dataDir>/jwt.secret.
func LoadOrCreateJWTSecret(dataDir, envSecret string) ([]byte, error) {
	if s := strings.TrimSpace(envSecret); s != "" {
		return []byte(s), nil
	}
	if dataDir == "" {
		buf := make([]byte, 32)
		_, _ = rand.Read(buf)
		return buf, nil
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	p := filepath.Join(dataDir, "jwt.secret")
	if raw, err := os.ReadFile(p); err == nil && len(strings.TrimSpace(string(raw))) >= 32 {
		return []byte(strings.TrimSpace(string(raw))), nil
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	hexSecret := hex.EncodeToString(buf)
	if err := os.WriteFile(p, []byte(hexSecret), 0o600); err != nil {
		return nil, err
	}
	return []byte(hexSecret), nil
}

func hashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func checkPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

func signJWT(secret []byte, userID, username string, ttl time.Duration) (string, error) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	claimsRaw, err := json.Marshal(jwtClaims{
		Sub: userID,
		Usr: username,
		Exp: time.Now().Add(ttl).Unix(),
	})
	if err != nil {
		return "", err
	}
	payload := base64.RawURLEncoding.EncodeToString(claimsRaw)
	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signingInput + "." + sig, nil
}

func verifyJWT(secret []byte, token string) (jwtClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return jwtClaims{}, errors.New("invalid token format")
	}
	signingInput := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(signingInput))
	expected := mac.Sum(nil)
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(expected, got) {
		return jwtClaims{}, errors.New("invalid token signature")
	}
	claimsBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return jwtClaims{}, err
	}
	var claims jwtClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return jwtClaims{}, err
	}
	if time.Now().Unix() > claims.Exp {
		return jwtClaims{}, errors.New("token expired")
	}
	return claims, nil
}
