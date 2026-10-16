package gamesheet

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

func fakeJWT(exp int64) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"exp":%d}`, exp)))
	return "eyJhbGciOiJSUzI1NiJ9." + payload + ".sig"
}

func TestJWTExpired(t *testing.T) {
	if jwtExpired(fakeJWT(time.Now().Add(time.Hour).Unix())) {
		t.Error("token expiring in an hour reported as expired")
	}
	if !jwtExpired(fakeJWT(time.Now().Add(30 * time.Second).Unix())) {
		t.Error("token expiring in 30s should be treated as expired (safety margin)")
	}
	if !jwtExpired(fakeJWT(time.Now().Add(-time.Hour).Unix())) {
		t.Error("expired token reported as valid")
	}
	if !jwtExpired("garbage") {
		t.Error("undecodable token should be treated as expired")
	}
}
