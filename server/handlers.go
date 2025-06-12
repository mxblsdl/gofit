package server

import (
	"bytes"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/a-h/templ"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/gofit/models"
	"github.com/gofit/templates"
)

// HTTP handlers
func LineChartHandler(w http.ResponseWriter, r *http.Request) {
	chartType := r.URL.Query().Get("type")

	var data models.ChartData
	switch chartType {
	case "steps":
		data = models.Store.StepsData
	case "calories":
		data = models.Store.CaloriesData
	default:
		// data = Store.GetHeartRateData()
	}
	line := generateLineChart(data)
	log.Println("Generating line chart for type:", chartType)

	var buf bytes.Buffer
	err := line.Render(&buf)
	if err != nil {
		http.Error(w, "Failed to render chart", http.StatusInternalServerError)
		return
	}

	component := templates.LineChart(template.HTML(buf.String()), data.Title)
	templ.Handler(component).ServeHTTP(w, r)
}

// TODO move to separate file
func generateLineChart(data models.ChartData) *charts.Line {
	line := charts.NewLine()

	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: "macarons"}),
		charts.WithTitleOpts(opts.Title{
			Title:    data.Title,
			Subtitle: data.Subtitle,
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:            opts.Bool(true),
			Trigger:         "item",
			BackgroundColor: "#f5f5f5",
			BorderColor:     "#ccc",
			AxisPointer: &opts.AxisPointer{
				Type: "cross",
			}}),
		// charts.WithXAxisOpts(opts.XAxis{Data: data.XAxis}),
	)

	// X-axis data
	line.SetXAxis(data.XAxis)

	// Add each series from the data
	for name, values := range data.Series {
		line.AddSeries(name, generateLineItems(values))
	}

	line.SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: opts.Bool(true)}))

	return line
}

// generateLineItems converts int slice to LineData slice
func generateLineItems(data []int) []opts.LineData {
	items := make([]opts.LineData, 0)
	for _, v := range data {
		items = append(items, opts.LineData{Value: v})
	}
	return items
}

func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	profileData := models.Store.ProfileData

	// Render the profile template with the profile data
	component := templates.Profile(profileData)
	templ.Handler(component).ServeHTTP(w, r)
}

func AuthHandler(w http.ResponseWriter, r *http.Request) {
	component := templates.Auth()
	templ.Handler(component).ServeHTTP(w, r)
}

func AuthSubmitHandler(w http.ResponseWriter, r *http.Request) {
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
		// RedirectURI:  "http://localhost:8080",
	}

	// write the credentials to a file or database
	filePath := filepath.Join("fitbit_data", "account_info.json")
	account_data, err := json.MarshalIndent(account_info, "", "  ")
	os.WriteFile(filePath, account_data, 0644)

	err = models.PopulateDataStore(account_info.ClientID, account_info.ClientSecret, "fitbit_data")
	if err != nil {
		http.Error(w, "Failed to populate data store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Data store populated successfully")

	// Respond to the client
	w.Header().Set("Content-Type", "text/html")
	templ.Handler(templates.Index()).ServeHTTP(w, r)
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	if models.AuthFlowInProgress {
		log.Println("Auth flow in progress, redirecting to auth page")
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}

	account_info_file := filepath.Join("fitbit_data", "account_info.json")
	if _, err := os.Stat(account_info_file); os.IsNotExist(err) {
		// If account_info does not exist, redirect to the auth page
		log.Println("Account info not found, redirecting to auth page")
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}

	data, err := os.ReadFile(account_info_file)
	if err != nil {
		http.Error(w, "Failed to read account info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var account_info models.Config
	err = json.Unmarshal(data, &account_info)
	if err != nil {
		http.Error(w, "Failed to parse account info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Println("Account info loaded successfully:", account_info)

	err = models.PopulateDataStore(account_info.ClientID, account_info.ClientSecret, "fitbit_data")
	if err != nil {
		component := templates.Error("Failed to populate data store: " + err.Error())
		templ.Handler(component).ServeHTTP(w, r)
		return
	}

	component := templates.Index()
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
