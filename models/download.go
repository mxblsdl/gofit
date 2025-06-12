package models

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

// TODO simplify or generalize this, maybe rename?
var Store = DataStore{
	StepsData:    ChartData{},
	CaloriesData: ChartData{},
	ProfileData:  ProfileData{},
}

const DAYS_BACK int = 5

// NewFitbitDownloader creates a new downloader instance
func NewFitbitDownloader(clientID, clientSecret, dataDir string) *FitbitDownloader {
	// Create data directory if it doesn't exist
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		err := os.Mkdir(dataDir, 0755)
		if err != nil {
			log.Fatal("Failed to create data directory:", err)
		}
	}

	return &FitbitDownloader{
		Config: Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURI:  "http://localhost:8080",
			RedirectPort: "8080",
		},
		DataDir: dataDir,
	}
}

func PopulateDataStore(clientID, clientSecret, dataDir string) error {
	downloader := NewFitbitDownloader(clientID, clientSecret, dataDir)

	// Check if we already have token information
	err := downloader.LoadTokenInfo()
	if err != nil {
		// First time authentication (only needed once)
		// This will open your browser for authorization
		fmt.Println("No token information found. Starting authorization flow...")
		err = downloader.StartAuthFlow()
		if err != nil {
			fmt.Println("Authorization failed:", err)
			return err
		}
	} else {
		// Refresh the access token if it exists
		fmt.Println("Refreshing access token...")
		err = downloader.RefreshAccessToken()
		if err != nil {
			fmt.Println("Failed to refresh access token:", err)
			return err
		}
	}

	profileData, err := downloader.DownloadProfile()
	if err != nil {
		log.Fatal("Failed to download profile:", err)
	}
	if profileData != nil {
		Store.ProfileData = *profileData
	}

	stepData, err := downloader.DownloadActivities("steps", DAYS_BACK)
	if err != nil {
		log.Fatal("Failed to download steps data:", err)
	}
	if stepData != nil {
		processedData := stepData.ProcessData(strconv.Itoa(DAYS_BACK))
		Store.StepsData = processedData
	}

	caloriesData, err := downloader.DownloadActivities("calories", DAYS_BACK)
	if err != nil {
		log.Fatal("Failed to download calories data:", err)
	}
	if caloriesData != nil {
		processedData := caloriesData.ProcessData(strconv.Itoa(DAYS_BACK))
		Store.CaloriesData = processedData
	}
	return nil

}
