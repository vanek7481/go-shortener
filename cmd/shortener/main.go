package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"shortener/internal/app"
)

// A fixed secret is acceptable for this educational project only.
const secretKey = "super-secret-key"

func main() {
	server := &http.Server{
		Addr:              ":8080",
		Handler:           app.New("http://localhost:8080", secretKey),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	log.Print("URL shortener listening on http://localhost:8080")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
