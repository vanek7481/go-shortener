package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSignedCookie(t *testing.T) {
	a := New("secret")
	w := httptest.NewRecorder()
	id, err := a.User(w, httptest.NewRequest("GET", "https://example.com", nil), false)
	if err != nil {
		t.Fatal(err)
	}
	cookie := w.Result().Cookies()[0]
	if !cookie.Secure {
		t.Fatal("HTTPS cookie must be secure")
	}
	h := hmac.New(sha256.New, []byte("secret"))
	h.Write([]byte(id))
	if cookie.Value != id+":"+hex.EncodeToString(h.Sum(nil)) {
		t.Fatal("not an HMAC-SHA256 signature")
	}
	got, valid, err := a.Verify(cookie.Value)
	if err != nil || !valid || got != id {
		t.Fatal("valid signature rejected")
	}
	for _, value := range []string{"changed:" + strings.Split(cookie.Value, ":")[1], id + ":" + strings.Repeat("0", 64)} {
		if _, valid, _ := a.Verify(value); valid {
			t.Fatal("tampered cookie accepted")
		}
	}
	if _, valid, _ := New("other-secret").Verify(cookie.Value); valid {
		t.Fatal("wrong key accepted")
	}
}
