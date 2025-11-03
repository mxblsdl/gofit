package server

import (
	"log"
	"net/http"
)

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Endpoint: %s, Method: %s", r.URL.Path, r.Method)
		next.ServeHTTP(w, r)
	})
}
