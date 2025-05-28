package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofit/models"
	"github.com/joho/godotenv"
)

// NewFitbitDownloader creates a new downloader instance
func NewFitbitDownloader(clientID, clientSecret string) *models.FitbitDownloader {
	dataDir := "fitbit_data"
	// Create data directory if it doesn't exist
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		err := os.Mkdir(dataDir, 0755)
		if err != nil {
			log.Fatal("Failed to create data directory:", err)
		}
	}

	return &models.FitbitDownloader{
		Config: models.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURI:  "http://localhost:8080",
		},
		DataDir: dataDir,
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Replace with your own Client ID and Client Secret from Fitbit Developer Portal
	clientID := os.Getenv("FITBIT_ID")
	clientSecret := os.Getenv("FITBIT_SECRET")

	downloader := NewFitbitDownloader(clientID, clientSecret)

	// Check if we already have token information
	err = downloader.LoadTokenInfo()
	if err != nil {
		// First time authentication (only needed once)
		// This will open your browser for authorization
		fmt.Println("No token information found. Starting authorization flow...")
		err = downloader.StartAuthFlow()
		if err != nil {
			log.Fatal("Authorization failed:", err)
		}
	}

	// Download all data for the last 30 days
	err = downloader.DownloadAllData(30)
	if err != nil {
		log.Fatal("Failed to download data:", err)
	}
}
