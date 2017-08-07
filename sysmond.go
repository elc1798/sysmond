package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/elc1798/sysmon"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	MONITORS = []sysmon.Monitor{
		&sysmon.CPUMonitor{},
		&sysmon.MemoryMonitor{},
		&sysmon.LoadMonitor{},
	}

	MONITOR_FIELD_METRICS = make(map[string]prometheus.Collector)
)

func getFieldKey(mon sysmon.Monitor, field string) string {
	return fmt.Sprintf("%v_%v", mon.Name(), field)
}

func getEvaluator(mon sysmon.Monitor, field string) func() float64 {
	return func() float64 { return mon.GetValue(field) }
}

func startMonitor(mon sysmon.Monitor, ticker *time.Ticker) {
	log.Printf("Starting Monitor '%v'", mon.Name())

	// Initialize monitor
	mon.Init()

	// Register monitor error metric
	log.Printf("Registering error metric for '%v'", mon.Name())
	errMetric := prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: mon.Name(),
		Name:      "errors",
		Help:      fmt.Sprintf("Errors for Monitor(%v)", mon.Name()),
	})
	prometheus.MustRegister(errMetric)

	// Register uptime metric
	log.Printf("Registering metric for '%v_uptime'", mon.Name())
	uptimeMetric := prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Subsystem: mon.Name(),
			Name:      "uptime",
			Help:      fmt.Sprintf("Uptime for %v", mon.Name()),
		}, func() float64 {
			return mon.GetUptime().Seconds()
		},
	)
	prometheus.MustRegister(uptimeMetric)

	// Build the field channels and register metrics
	fields := mon.GetFields()
	for _, field := range fields {
		fieldKey := getFieldKey(mon, field)
		log.Printf("Registering metric for '%v'", fieldKey)

		MONITOR_FIELD_METRICS[fieldKey] = prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Subsystem: mon.Name(),
				Name:      field,
				Help:      fmt.Sprintf("%v as reported by %v", field, mon.Name()),
			}, getEvaluator(mon, field),
		)

		if err := prometheus.Register(MONITOR_FIELD_METRICS[fieldKey]); err != nil {
			log.Fatal("Failed to register metric : '%v'", fieldKey)
		}
	}

	// Update on constant ticker
	for {
		if err := mon.UpdateValues(); err != nil {
			errMetric.Inc()
		}

		// Advance ticker
		<-ticker.C
	}
}

func startMonitors() {
	for _, mon := range MONITORS {
		ticker := time.NewTicker(time.Millisecond * 500)
		go startMonitor(mon, ticker)
	}
}

func main() {
	startMonitors()

	log.Println("Starting metrics HTTP endpoint")
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8747", nil))
}
