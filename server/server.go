package server

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"net/http"

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
	// This is a placeholder for the authentication handler
	// In a real application, you would redirect to Fitbit's OAuth flow here
	fmt.Println("test")
}

func Serve() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Set up HTTP routes
	http.Handle("/", templ.Handler(templates.Landing()))

	http.HandleFunc("/auth", AuthHandler)
	// http.Handle("/", templ.Handler(templates.Index()))
	http.HandleFunc("/profile", ProfileHandler)

	http.HandleFunc("/line", LineChartHandler)
	// http.Handle("/line", templ.Handler(templates.LineChart()))
	// http.HandleFunc("/bar", barChartHandler)

	// Chart api endpoints
	// http.HandleFunc("/api/line", apiLineChartHandler)

	port := "8080"
	log.Printf("Server starting on http://localhost:%s", port)
	log.Printf("Visit http://localhost:%s to see the charts", port)

	// Start the server
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
