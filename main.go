package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofit/models"
	"github.com/gofit/server"
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
	DAYS_BACK := 5

	clientID := os.Getenv("FITBIT_ID")
	clientSecret := os.Getenv("FITBIT_SECRET")

	downloader := NewFitbitDownloader(clientID, clientSecret)
	// err = downloader.ClearAllData()
	if err != nil {
		log.Fatal("Failed to clear existing data:", err)
	}

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

	endDate := time.Now().Format("2006-01-02")
	startDate := time.Now().AddDate(0, 0, -DAYS_BACK).Format("2006-01-02")

	profileData, err := downloader.DownloadProfile()
	if err != nil {
		log.Fatal("Failed to download profile:", err)
	}
	if profileData != nil {
		server.Store.ProfileData = *profileData
	}

	stepData, err := downloader.DownloadActivities("steps", startDate, endDate)
	if err != nil {
		log.Fatal("Failed to download steps data:", err)
	}
	if stepData != nil {
		processedData := stepData.ProcessData()
		server.Store.StepsData = processedData
	}

	// Downloading other activities to check their format
	_, err = downloader.DownloadActivities("calories", startDate, endDate)

	server.Serve()
}
