package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
)

const CookieName = "user_id"

var ErrMalformedCookie = errors.New("cookie does not contain a valid user ID and signature")

type Auth struct {
	key []byte
}

func New(secret string) *Auth { return &Auth{key: []byte(secret)} }

func (a *Auth) signature(id string) []byte {
	h := hmac.New(sha256.New, a.key)
	h.Write([]byte(id))
	return h.Sum(nil)
}

// Verify separates malformed values from well-formed values with invalid signatures.
func (a *Auth) Verify(value string) (string, bool, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false, ErrMalformedCookie
	}
	signature, err := hex.DecodeString(parts[1])
	if err != nil || len(signature) != sha256.Size {
		return "", false, ErrMalformedCookie
	}
	return parts[0], hmac.Equal(a.signature(parts[0]), signature), nil
}

// User reuses authentic cookies or creates a new identity. Strict mode rejects
// malformed cookies, as required by GET /api/user/urls.
func (a *Auth) User(w http.ResponseWriter, r *http.Request, strict bool) (string, error) {
	cookie, err := r.Cookie(CookieName)
	if err == nil {
		id, valid, verifyErr := a.Verify(cookie.Value)
		if strict && verifyErr != nil {
			return "", verifyErr
		}
		if verifyErr == nil && valid {
			return id, nil
		}
	} else if !errors.Is(err, http.ErrNoCookie) {
		return "", err
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	id := hex.EncodeToString(random[:])
	http.SetCookie(w, &http.Cookie{
		Name: CookieName, Value: id + ":" + hex.EncodeToString(a.signature(id)),
		Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode,
	})
	return id, nil
}
