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
	http.Handle("/", loggingMiddleware(http.HandlerFunc(IndexHandler)))
	http.Handle("/auth", loggingMiddleware(http.HandlerFunc(AuthHandler)))
	http.Handle("/auth-submit", loggingMiddleware(http.HandlerFunc(AuthSubmitHandler)))
	http.Handle("/profile", loggingMiddleware(http.HandlerFunc(ProfileHandler)))
	http.Handle("/line", loggingMiddleware(http.HandlerFunc(LineChartHandler)))
	http.Handle("/remove-secrets", loggingMiddleware(http.HandlerFunc(removeSecretsHandler)))

	port := "8081"
	log.Printf("Server starting on http://localhost:%s", port)
	log.Printf("Visit http://localhost:%s to see the charts", port)

	// Start the server
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

// Add another activity type
// Play around with chart options
// Look into conditional rendering of buttons in nav bar
// Convert profile height and weight to US customary units
