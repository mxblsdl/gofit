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

func Serve() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Set up HTTP routes
	http.Handle("/", loggingMiddleware(http.HandlerFunc(indexHandler)))
	http.Handle("/auth", loggingMiddleware(http.HandlerFunc(authHandler)))
	http.Handle("/auth-submit", loggingMiddleware(http.HandlerFunc(authSubmitHandler)))
	http.Handle("/profile", loggingMiddleware(http.HandlerFunc(profileHandler)))
	http.Handle("/remove-secrets", loggingMiddleware(http.HandlerFunc(removeSecretsHandler)))
	http.Handle("/update-days", loggingMiddleware(http.HandlerFunc(updateDaysHandler)))

	port := "8081"
	log.Printf("Server starting on http://localhost:%s", port)
	log.Printf("Visit http://localhost:%s to see the charts", port)

	// Start the server
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
