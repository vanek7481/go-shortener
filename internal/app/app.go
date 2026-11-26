package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"

	"shortener/internal/auth"
	"shortener/internal/storage"
)

type App struct {
	baseURL string
	auth    *auth.Auth
	store   *storage.Memory
}

func New(baseURL, secret string) http.Handler {
	a := &App{baseURL: strings.TrimRight(baseURL, "/"), auth: auth.New(secret), store: storage.New()}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/shorten", a.shorten)
	mux.HandleFunc("/api/user/urls", a.list)
	mux.HandleFunc("/", a.redirect)
	return mux
}

func method(w http.ResponseWriter, r *http.Request, allowed string) bool {
	if r.Method == allowed {
		return true
	}
	w.Header().Set("Allow", allowed)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return false
}

func (a *App) user(w http.ResponseWriter, r *http.Request, strict bool) (string, bool) {
	id, err := a.auth.User(w, r, strict)
	if errors.Is(err, auth.ErrMalformedCookie) {
		http.Error(w, "malformed user cookie", http.StatusUnauthorized)
		return "", false
	}
	if err != nil {
		http.Error(w, "could not identify user", http.StatusInternalServerError)
		return "", false
	}
	return id, true
}

func writeJSON(w http.ResponseWriter, data any) {
	// Marshal before committing headers, so encoding errors have an HTTP response.
	encoded, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "could not encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded) // A disconnected client cannot receive another error.
}

func (a *App) shorten(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodPost) {
		return
	}
	userID, ok := a.user(w, r, false)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	var request struct {
		URL string `json:"url"`
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "expected JSON with a url field", http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		http.Error(w, "expected a single JSON object", http.StatusBadRequest)
		return
	}
	u, err := url.ParseRequestURI(request.URL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || strings.ContainsAny(request.URL, "\r\n") {
		http.Error(w, "url must be an absolute HTTP or HTTPS URL", http.StatusBadRequest)
		return
	}
	id := a.store.Create(userID, request.URL)
	writeJSON(w, struct {
		Result string `json:"result"`
	}{a.baseURL + "/" + id})
}

func (a *App) redirect(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	if !method(w, r, http.MethodGet) {
		return
	}
	if _, ok := a.user(w, r, false); !ok {
		return
	}
	original, ok := a.store.Get(id)
	if !ok {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, original, http.StatusTemporaryRedirect)
}

func (a *App) list(w http.ResponseWriter, r *http.Request) {
	if !method(w, r, http.MethodGet) {
		return
	}
	userID, ok := a.user(w, r, true)
	if !ok {
		return
	}
	links := a.store.List(userID)
	if len(links) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	type userURL struct {
		ShortURL    string `json:"short_url"`
		OriginalURL string `json:"original_url"`
	}
	result := make([]userURL, 0, len(links))
	for _, link := range links {
		result = append(result, userURL{a.baseURL + "/" + link.ID, link.OriginalURL})
	}
	writeJSON(w, result)
}
