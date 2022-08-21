package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	INSERT_METRIC = "insert"
	READ_METRIC   = "read"
)

var (
	dataBaseMetrics = map[string]*prometheus.GaugeVec{
		INSERT_METRIC: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "database_insert",
			Help: "time (in seconds) taken to insert X elements on the database",
		}, []string{
			"elements",
		}),
		READ_METRIC: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "database_read",
			Help: "time (in seconds) taken to read X elements on the database",
		}, []string{
			"elements",
		}),
	}
)
