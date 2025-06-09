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

// TODO simplify or generalize this, maybe rename?
var Store = models.DataStore{
	StepsData:   models.ChartData{},
	ProfileData: models.ProfileData{},
}

// HTTP handlers
func LineChartHandler(w http.ResponseWriter, r *http.Request) {
	chartType := r.URL.Query().Get("type")

	var data models.ChartData
	switch chartType {
	case "steps":
		data = Store.GetStepsData()
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
	profileData := Store.ProfileData

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
	}

	// write the credentials to a file or database
	filePath := filepath.Join("fitbit_data", "credentials.json")
	account_data, err := json.MarshalIndent(account_info, "", "  ")
	os.WriteFile(filePath, account_data, 0644)

	// fmt.Printf("Received Fitbit ID: %s, Secret: %s\n", fitbitID, fitbitSecret)

	// Respond to the client
	// TODO create a new downloader instance and start data pulls
	w.Header().Set("Content-Type", "text/html")
	templ.Handler(templates.Index()).ServeHTTP(w, r)
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	account_info := filepath.Join("fitbit_data", "account_info.json")
	if _, err := os.Stat(account_info); os.IsNotExist(err) {
		// If account_info does not exist, redirect to the auth page
		http.Redirect(w, r, "/auth", http.StatusFound)
		return
	}

	// Render the index template
	component := templates.Index()
	templ.Handler(component).ServeHTTP(w, r)
}

func Serve() {

	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Set up HTTP routes
	http.HandleFunc("/", IndexHandler)

	http.HandleFunc("/auth", AuthHandler)
	http.HandleFunc("/auth-submit", AuthSubmitHandler)
	// http.Handle("/", templ.Handler(templates.Index()))
	http.HandleFunc("/profile", ProfileHandler)

	http.HandleFunc("/line", LineChartHandler)
	// http.HandleFunc("/bar", barChartHandler)

	port := "8080"
	log.Printf("Server starting on http://localhost:%s", port)
	log.Printf("Visit http://localhost:%s to see the charts", port)

	// Start the server
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

// TODO: check for id and secret file
// if not found, redirect to auth page
// save the id and secret to a file

// TODO: remove nav bar from the landing page

// TODO: make sure endpoint point to correct templates, currently redirects to auth landing page
