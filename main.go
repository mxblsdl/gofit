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

	// Replace with your own Client ID and Client Secret from Fitbit Developer Portal
	clientID := os.Getenv("FITBIT_ID")
	clientSecret := os.Getenv("FITBIT_SECRET")

	downloader := NewFitbitDownloader(clientID, clientSecret)
	err = downloader.ClearAllData()
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

	// Download all data for the last 30 days
	// err = downloader.DownloadAllData(30)
	endDate := time.Now().Format("2006-01-02")
	startDate := time.Now().AddDate(0, 0, -DAYS_BACK).Format("2006-01-02")

	err = downloader.DownloadProfile()
	if err != nil {
		log.Fatal("Failed to download profile:", err)
	}

	// heartData, err := downloader.DownloadActivities("heart", startDate, endDate)
	// if err != nil {
	// 	log.Fatal("Failed to download heart rate data:", err)
	// }
	stepData, err := downloader.DownloadActivities("steps", startDate, endDate)
	if err != nil {
		log.Fatal("Failed to download steps data:", err)
	}

	// Print out the data for debugging
	if stepData != nil {
		stepsDataObj, ok := stepData.(models.StepsData)
		if !ok {
			log.Fatal("Failed to convert data to StepsData tyep")
		}
		processedData := stepsDataObj.ProcessData()

		server.Store.StepsData = processedData
	}

	server.Serve()
}
