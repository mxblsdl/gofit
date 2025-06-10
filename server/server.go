package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/a-h/templ"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/gofit/downloader"
	"github.com/gofit/models"
	"github.com/gofit/templates"
)

// HTTP handlers
func LineChartHandler(w http.ResponseWriter, r *http.Request) {
	chartType := r.URL.Query().Get("type")

	var data models.ChartData
	switch chartType {
	case "steps":
		data = downloader.Store.GetStepsData()
	default:
		// data = Store.GetHeartRateData()
	}
	line := generateLineChart(data)

	var buf bytes.Buffer
	err := line.Render(&buf)
	if err != nil {
		http.Error(w, "Failed to render chart", http.StatusInternalServerError)
		return
	}

	component := templates.LineChart(template.HTML(buf.String()), data.Title)
	templ.Handler(component).ServeHTTP(w, r)
}

// generateLineChart creates a sample line chart
func generateLineChart(data models.ChartData) *charts.Line {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: "macarons"}),
		charts.WithTitleOpts(opts.Title{
			Title:    data.Title,
			Subtitle: data.Subtitle,
		}),
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
	profileData := downloader.Store.ProfileData

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

	// fmt.Printf("Received Fitbit ID: %s, Secret: %s\n", fitbitID, fitbitSecret)
	err = downloader.PopulateDataStore(account_info.ClientID, account_info.ClientSecret, "fitbit_data")
	if err != nil {
		http.Error(w, "Failed to populate data store: "+err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Println("Data store populated successfully")

	// Respond to the client
	w.Header().Set("Content-Type", "text/html")
	templ.Handler(templates.Index()).ServeHTTP(w, r)
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	account_info_file := filepath.Join("fitbit_data", "account_info.json")
	if _, err := os.Stat(account_info_file); os.IsNotExist(err) {
		// If account_info does not exist, redirect to the auth page
		log.Println("Account info not found, redirecting to auth page")
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	} else {
		// If account_info exists, read it
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

		fmt.Println("Account info loaded successfully:", account_info)
		err = downloader.PopulateDataStore(account_info.ClientID, account_info.ClientSecret, "fitbit_data")
		if err != nil {
			component := templates.Error("Failed to populate data store: " + err.Error())
			templ.Handler(component).ServeHTTP(w, r)
			return
		}

	}

	// Render the index template
	component := templates.Index()
	templ.Handler(component).ServeHTTP(w, r)
}

func removeSecretsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.Method)

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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Endpoint: %s, Method: %s", r.URL.Path, r.Method)
		next.ServeHTTP(w, r) // Call the original handler
	})
}

func Serve() {

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Set up HTTP routes
	http.Handle("/", loggingMiddleware(http.HandlerFunc(IndexHandler)))
	http.Handle("/auth", loggingMiddleware(http.HandlerFunc(AuthHandler)))
	http.Handle("/auth-submit", loggingMiddleware(http.HandlerFunc(AuthSubmitHandler)))
	http.Handle("/profile", loggingMiddleware(http.HandlerFunc(ProfileHandler)))
	http.Handle("/line", loggingMiddleware(http.HandlerFunc(LineChartHandler)))
	http.Handle("/remove-secrets", loggingMiddleware(http.HandlerFunc(removeSecretsHandler)))

	port := "8080"
	log.Printf("Server starting on http://localhost:%s", port)
	log.Printf("Visit http://localhost:%s to see the charts", port)

	// Start the server
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

// TODO: utilize other data from the downloader
// TODO: some type of error when client secret and id dont work
// TODO: change how charts look at little

// Set days back to be a variable??
