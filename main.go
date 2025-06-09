package main

import (
	"log"

	"github.com/gofit/server"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// clientID := os.Getenv("FITBIT_ID")
	// clientSecret := os.Getenv("FITBIT_SECRET")
	// dataDir := os.Getenv("DATA_DIR")

	// downloader := downloader.NewFitbitDownloader(clientID, clientSecret, dataDir)

	// Downloading other activities to check their format
	// _, err = downloader.DownloadActivities("calories", startDate, endDate)

	server.Serve()
}
