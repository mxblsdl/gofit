package server

import (
	"log"
	"net/http"
)

func Serve() {

	logFile, err := setupLogging()
	if err != nil {
		log.Fatalf("Failed to set up logging: %v", err)
	}
	defer logFile.Close()

	// Serve static files
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
	host := "0.0.0.0"

	log.Printf("Server starting on http://%s:%s", host, port)
	log.Printf("Visit http://%s:%s to see the charts", host, port)
	if err := http.ListenAndServe(host+":"+port, nil); err != nil {
		log.Printf("Could not listen on %s: %v\n", port, err)
	}
}
