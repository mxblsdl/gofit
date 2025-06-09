package downloader

import (
	"log"
	"os"

	"github.com/gofit/models"
)

// NewFitbitDownloader creates a new downloader instance
func NewFitbitDownloader(clientID, clientSecret, dataDir string) *models.FitbitDownloader {
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
