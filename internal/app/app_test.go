package app_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"shortener/internal/app"
	"shortener/internal/auth"
)

func request(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func create(t *testing.T, h http.Handler, original string, cookie *http.Cookie) (string, *http.Cookie) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"url": original})
	if err != nil {
		t.Fatal(err)
	}
	w := request(h, "POST", "/api/shorten", string(body), cookie)
	if w.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", w.Code, w.Body)
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		t.Fatal("missing JSON content type")
	}
	var result struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(result.Result)
	if err != nil || u.Host != "localhost:8080" || u.Path == "" {
		t.Fatalf("invalid short URL: %s", result.Result)
	}
	if cookie == nil {
		cookies := w.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("expected a cookie, got %v", cookies)
		}
		cookie = cookies[0]
		if cookie.Name != auth.CookieName || !cookie.HttpOnly || cookie.Path != "/" {
			t.Fatalf("unexpected cookie: %v", cookie)
		}
	}
	return result.Result, cookie
}

func TestCreateRedirectAndUserIsolation(t *testing.T) {
	h := app.New("http://localhost:8080", "test-secret")
	first, cookie := create(t, h, "https://example.com/long?a=1&b=2", nil)
	second, _ := create(t, h, "https://example.com/second", cookie)
	if first == second {
		t.Fatal("identifiers collide")
	}
	_, other := create(t, h, "https://other.example/private", nil)
	if cookie.Value == other.Value {
		t.Fatal("users have identical IDs")
	}
	w := request(h, "GET", strings.TrimPrefix(first, "http://localhost:8080"), "", nil)
	if w.Code != http.StatusTemporaryRedirect || w.Header().Get("Location") != "https://example.com/long?a=1&b=2" {
		t.Fatalf("redirect: %d %v", w.Code, w.Header())
	}
	for _, tc := range []struct {
		cookie *http.Cookie
		want   []string
	}{
		{cookie, []string{"https://example.com/long?a=1&b=2", "https://example.com/second"}},
		{other, []string{"https://other.example/private"}},
	} {
		w = request(h, "GET", "/api/user/urls", "", tc.cookie)
		if w.Code != http.StatusOK {
			t.Fatalf("list status=%d", w.Code)
		}
		var links []struct {
			Short    string `json:"short_url"`
			Original string `json:"original_url"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &links); err != nil {
			t.Fatal(err)
		}
		if len(links) != len(tc.want) {
			t.Fatalf("links=%v", links)
		}
		for i, link := range links {
			if link.Original != tc.want[i] || !strings.HasPrefix(link.Short, "http://localhost:8080/") {
				t.Fatalf("unexpected link: %+v", link)
			}
		}
		if len(w.Result().Cookies()) != 0 {
			t.Fatal("valid cookie was replaced")
		}
	}
}

func TestCookieResponses(t *testing.T) {
	h := app.New("http://localhost:8080", "test-secret")
	_, valid := create(t, h, "https://example.com", nil)
	parts := strings.Split(valid.Value, ":")
	tampered := "different-user:" + parts[1]
	for _, tc := range []struct {
		name      string
		cookie    *http.Cookie
		status    int
		newCookie bool
	}{
		{"absent", nil, 204, true},
		{"empty", &http.Cookie{Name: auth.CookieName}, 401, false},
		{"missing ID", &http.Cookie{Name: auth.CookieName, Value: ":" + parts[1]}, 401, false},
		{"bad format", &http.Cookie{Name: auth.CookieName, Value: "broken"}, 401, false},
		{"bad hex", &http.Cookie{Name: auth.CookieName, Value: "id:xyz"}, 401, false},
		{"tampered", &http.Cookie{Name: auth.CookieName, Value: tampered}, 204, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request(h, "GET", "/api/user/urls", "", tc.cookie)
			if w.Code != tc.status {
				t.Fatalf("status=%d want=%d", w.Code, tc.status)
			}
			if (len(w.Result().Cookies()) > 0) != tc.newCookie {
				t.Fatal("unexpected cookie issuance")
			}
			if tc.status == 204 && w.Body.Len() != 0 {
				t.Fatal("204 must not have a body")
			}
			if tc.newCookie {
				w = request(h, "GET", "/api/user/urls", "", w.Result().Cookies()[0])
				if w.Code != 204 || len(w.Result().Cookies()) != 0 {
					t.Fatal("empty user cookie was not reused")
				}
			}
		})
	}
	for _, value := range []string{"", "broken", tampered} {
		w := request(h, "POST", "/api/shorten", `{"url":"https://example.com"}`, &http.Cookie{Name: auth.CookieName, Value: value})
		if w.Code != 200 || len(w.Result().Cookies()) != 1 {
			t.Fatalf("invalid POST cookie was not replaced: %d", w.Code)
		}
	}
}

func TestInvalidRequests(t *testing.T) {
	h := app.New("http://localhost:8080", "test-secret")
	for _, body := range []string{"", "{", "null", "[]", `{}`, `{"url":5}`, `{"url":"/relative"}`, `{"url":"javascript:alert(1)"}`, `{"url":"https://"}`, `{"url":"https://example.com"} {}`, strings.Repeat("x", (1<<20)+1)} {
		w := request(h, "POST", "/api/shorten", body, nil)
		if w.Code != 400 {
			t.Errorf("invalid request returned %d", w.Code)
		}
	}
	for _, path := range []string{"/missing", "/", "/api/unknown", "/a/b"} {
		if w := request(h, "GET", path, "", nil); w.Code != 404 {
			t.Errorf("%s status=%d", path, w.Code)
		}
	}
	for _, tc := range []struct{ method, path, allow string }{{"GET", "/api/shorten", "POST"}, {"POST", "/api/user/urls", "GET"}, {"DELETE", "/123", "GET"}} {
		w := request(h, tc.method, tc.path, "", nil)
		if w.Code != 405 || w.Header().Get("Allow") != tc.allow {
			t.Errorf("unexpected method response: %d", w.Code)
		}
	}
}
