package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	INSERT_METRIC = "insert"
	READ_METRIC   = "read"
	CPU_METRIC    = "cpu"
	MEM_METRIC    = "memory"
)

var (
	dataBaseMetrics = map[string]*prometheus.GaugeVec{
		INSERT_METRIC: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "database_insert",
			Help: "time (in seconds) taken to insert X elements on the database",
		}, []string{
			"elements",
			"replicas",
		}),
		READ_METRIC: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "database_read",
			Help: "time (in seconds) taken to read X elements on the database",
		}, []string{
			"elements",
			"replicas",
		}),
	}
	systemMetrics = map[string]*prometheus.GaugeVec{
		CPU_METRIC: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "system_cpu",
			Help: "cpu consuptiom in percentage of a given operation",
		}, []string{
			"replicas",
			"operation",
		}),
		MEM_METRIC: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "system_memory",
			Help: "memory usage of a given operation",
		}, []string{
			"replicas",
			"operation",
		}),
	}
)
