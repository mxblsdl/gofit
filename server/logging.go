package server

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

func setupLogging() (*os.File, error) {
	// Create logs directory if it doesn't exist
	err := os.MkdirAll("logs", 0755)
	if err != nil {
		return nil, err
	}

	// Create log file with timestamp
	logFileName := filepath.Join("logs", "app.log")
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	// Check if running with air
	if os.Getenv("AIR_RESTART_COUNT") != "" {
		log.SetOutput(logFile)
	} else {
		// Set log output to both file and console
		mw := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(mw)
	}

	// Set log format to include timestamp
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return logFile, nil
}
