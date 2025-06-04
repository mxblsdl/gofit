package server

import (
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
	StepsData: models.ChartData{},
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

// generateBarChart creates a sample bar chart
func generateBarChart() *charts.Bar {
	bar := charts.NewBar()
	bar.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: "macarons"}),
		charts.WithTitleOpts(opts.Title{
			Title:    "Sample Bar Chart",
			Subtitle: "Monthly Sales Data",
		}),
	)

	// X-axis data
	months := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"}

	bar.SetXAxis(months).
		AddSeries("Sales", generateBarItems([]int{2340, 1890, 2890, 2340, 2890, 2890}))

	return bar
}

// generateBarItems converts int slice to BarData slice
func generateBarItems(data []int) []opts.BarData {
	items := make([]opts.BarData, 0)
	for _, v := range data {
		items = append(items, opts.BarData{Value: v})
	}
	return items
}

// HTTP handlers
func lineChartHandler(w http.ResponseWriter, r *http.Request) {
	chartType := r.URL.Query().Get("type")

	var data models.ChartData
	switch chartType {
	case "steps":
		data = Store.GetStepsData()
	default:
		// data = Store.GetHeartRateData()
	}

	component := templates.LineChart(data)
	component.Render(r.Context(), w)
	// line.Render(w)
}

func barChartHandler(w http.ResponseWriter, r *http.Request) {
	bar := generateBarChart()
	bar.Render(w)
}

func Serve() {
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Set up HTTP routes
	http.Handle("/", templ.Handler(templates.Index()))
	http.Handle("/profile", templ.Handler(templates.Profile()))
	http.HandleFunc("/line", lineChartHandler)
	http.HandleFunc("/bar", barChartHandler)

	port := "8080"
	log.Printf("Server starting on http://localhost:%s", port)
	log.Printf("Visit http://localhost:%s to see the charts", port)

	// Start the server
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
