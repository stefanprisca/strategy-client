package tfc

import (
	"log"
	"net/http"

	"github.com/go-kit/kit/metrics/prometheus"
	promClient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var labelNames = []string{"TFC"}
var promeHist *prometheus.Histogram

func startProme() func() {

	srv := http.Server{Addr: ":9009"}
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		httpError := srv.ListenAndServe()
		if httpError != nil {
			log.Println("While serving HTTP: ", httpError)
			srv.Shutdown(nil)
		}
	}()

	promeHist = prometheus.NewHistogramFrom(
		promClient.HistogramOpts{
			Namespace: "tfc",
			Subsystem: "testing",
			Name:      "runtime",
			Help:      "No help",
		}, labelNames)

	return func() {
		srv.Shutdown(nil)
	}
}

func GetPlayerMetrics() *prometheus.Histogram {

	return promeHist
}

func GetPromLabels() []string {
	return labelNames
}
