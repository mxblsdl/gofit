package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/a-h/templ"
	"github.com/gofit/models"
	"github.com/gofit/templates"
)

// HTTP handlers
func profileHandler(w http.ResponseWriter, r *http.Request) {
	profileData := models.Store.ProfileData

	// Render the profile template with the profile data
	component := templates.Profile(profileData)
	templ.Handler(component).ServeHTTP(w, r)
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	component := templates.Auth()
	templ.Handler(component).ServeHTTP(w, r)
}

func authSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Parse the form data
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	// Get the form values
	account_info := models.Config{
		ClientID:     r.FormValue("fitbit_id"),
		ClientSecret: r.FormValue("fitbit_secret"),
	}

	// write the credentials to a file or database
	filePath := filepath.Join("fitbit_data", "account_info.json")
	account_data, err := json.MarshalIndent(account_info, "", "  ")
	if err != nil {
		log.Println("failed to indent account info data")
	}

	os.WriteFile(filePath, account_data, 0644)

	err = models.PopulateDataStore(account_info.ClientID, account_info.ClientSecret, "fitbit_data", 14)
	if err != nil {
		http.Error(w, "Failed to populate data store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Data store populated successfully")

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func findClientInfo(dataFolder string) (string, error) {

	account_info_file := filepath.Join(dataFolder, "account_info.json")
	if _, err := os.Stat(account_info_file); os.IsNotExist(err) {
		// If account_info does not exist, redirect to the auth page
		log.Println("Account info not found, redirecting to auth page")
		return "", err

	}
	return account_info_file, nil
}

func loadClientInfo(account_info_file string) (models.Config, error) {

	data, err := os.ReadFile(account_info_file)
	if err != nil {
		return models.Config{}, err
	}

	var account_info models.Config
	err = json.Unmarshal(data, &account_info)
	if err != nil {
		return models.Config{}, err
	}
	return account_info, nil
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if models.AuthFlowInProgress {
		log.Println("Auth flow in progress, redirecting to auth page")
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}

	account_info_file, err := findClientInfo("fitbit_data")
	if err != nil {
		log.Println("Account info not found, redirecting to auth page")
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}

	// Load client info
	account_info, err := loadClientInfo(account_info_file)
	if err != nil {
		component := templates.Error("Failed to load account info: " + err.Error())
		templ.Handler(component).ServeHTTP(w, r)
		return
	}

	log.Println("Account info loaded successfully:", account_info)

	err = models.PopulateDataStore(account_info.ClientID, account_info.ClientSecret, "fitbit_data", 14)
	if err != nil {
		component := templates.Error("Failed to populate data store: " + err.Error())
		templ.Handler(component).ServeHTTP(w, r)
		return
	}
	stepsChart := models.Store.StepsData.GenerateLineChart()

	eleChart := models.Store.ElevationData.GenerateLineChart()

	calChart := models.Store.CaloriesData.GenerateLineChart()

	heartChart := models.Store.HeartRateData.GenerateHeartRateChart()

	restingHeartChart := models.Store.HeartRateData.GenerateRestingHeartRateChart()

	component := templates.Index(
		template.HTML(stepsChart),
		template.HTML(eleChart),
		template.HTML(calChart),
		template.HTML(heartChart),
		template.HTML(restingHeartChart))
	templ.Handler(component).ServeHTTP(w, r)

}

func removeSecretsHandler(w http.ResponseWriter, r *http.Request) {
	account_info_file := filepath.Join("fitbit_data", "account_info.json")
	if _, err := os.Stat(account_info_file); os.IsNotExist(err) {
		http.Error(w, "Account info not found", http.StatusNotFound)
		return
	}
	// If account_info exists, remove it
	err := os.Remove(account_info_file)
	if err != nil {
		http.Error(w, "Failed to remove account info: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Account info removed successfully")
	http.Redirect(w, r, "/auth", http.StatusFound)
}

func updateDaysHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Update endpoint triggered")

	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}
	// parse form data
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	fmt.Printf("Debug form values: %s", r.Form)
	days_back := r.FormValue("days_back")

	log.Println("Updating days back to:", days_back)

	days, err := strconv.Atoi(days_back)
	if err != nil {
		http.Error(w, "Invalid days back value", http.StatusBadRequest)
		return
	}
	account_info_file, err := findClientInfo("fitbit_data")
	if err != nil {
		http.Error(w, "Account info not found: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Load client info
	account_info, err := loadClientInfo(account_info_file)
	if err != nil {
		http.Error(w, "Failed to load account info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	err = models.PopulateDataStore(account_info.ClientID, account_info.ClientSecret, "fitbit_data", days)
	if err != nil {
		http.Error(w, "Failed to populate data store: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Render charts
	stepsChart := models.Store.StepsData.GenerateLineChart()
	elevationChart := models.Store.ElevationData.GenerateLineChart()
	caloriesChart := models.Store.CaloriesData.GenerateLineChart()
	heartRateChart := models.Store.HeartRateData.GenerateHeartRateChart()
	restingHeartChart := models.Store.HeartRateData.GenerateRestingHeartRateChart()

	component := templates.Charts(
		template.HTML(stepsChart),
		template.HTML(elevationChart),
		template.HTML(caloriesChart),
		template.HTML(heartRateChart),
		template.HTML(restingHeartChart),
	)
	templ.Handler(component).ServeHTTP(w, r)
}
