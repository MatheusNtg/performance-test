package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	dataBaseMetrics = []prometheus.GaugeVec{
		*promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "insert_on_database_metric",
			Help: "time taken to insert X elements on the database",
		}, []string{
			"elements",
		}),
	}
)
