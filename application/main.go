package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gocarina/gocsv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

var (
	datasetPath           = "./dataset/neo.csv"
	databasePassword      = os.Getenv("MYSQL_ROOT_PASSWORD")
	databaseAddrs         = os.Getenv("MYSQL_SERVICE_ADDRS")
	databaseReplicas      = os.Getenv("DATABASE_REPLICAS")
	databaseName          = "my_database" // This is the default database created when we deploy the mySql
	tableName             = "nearest_objects"
	defaultTickerDuration = 100 * time.Millisecond
)

func readNObjectsFromDB(db *sql.DB, N int) {
	sqlStmt := fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, N)

	_, err := db.Query(sqlStmt)
	if err != nil {
		panic(err)
	}
}

func createObjects(db *sql.DB, objs []*DataObject) {
	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()

	sqlStmt := fmt.Sprintf(
		`INSERT INTO %s 
			(
				id, 
				name, 
				est_diameter_min, 
				est_diameter_max,
				relative_velocity,
				miss_distance,
				orbiting_body,
				sentry_object,
				absolute_magnitude,
				hazardous
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, tableName,
	)

	stmt, err := tx.Prepare(sqlStmt)
	if err != nil {
		panic(err)
	}
	defer stmt.Close()

	for _, obj := range objs {
		stmt.Exec(obj.Id, obj.Name, obj.MinDiameter, obj.MaxDiameter, obj.RelativeVelocity, obj.MissingDistance, obj.OrbitingBody, obj.SentryObject, obj.AbsoluteMagnitude, obj.Hazardous)
	}

	err = tx.Commit()
	if err != nil {
		panic(err)
	}
}

type DataObject struct {
	Id                int     `csv:"id"`
	Name              string  `csv:"name"`
	MinDiameter       float64 `csv:"est_diameter_min"`
	MaxDiameter       float64 `csv:"est_diameter_max"`
	RelativeVelocity  float64 `csv:"relative_velocity"`
	MissingDistance   float64 `csv:"miss_distance"`
	OrbitingBody      string  `csv:"orbiting_body"`
	SentryObject      bool    `csv:"sentry_object"`
	AbsoluteMagnitude float32 `csv:"absolute_magnitude"`
	Hazardous         bool    `csv:"hazardous"`
}

func getObjectsFromCsv() []*DataObject {
	in, err := os.Open(datasetPath)
	if err != nil {
		panic(err)
	}
	defer in.Close()

	objects := []*DataObject{}

	if err := gocsv.UnmarshalFile(in, &objects); err != nil {
		panic(err)
	}

	return objects
}

func connectToDb() *sql.DB {
	connectionURL := fmt.Sprintf("root:%s@tcp(%s)/%s", databasePassword, databaseAddrs, databaseName)
	db, err := sql.Open("mysql", connectionURL)
	if err != nil {
		panic(err)
	}
	err = db.Ping()
	if err != nil {
		panic(err)
	}
	fmt.Println("Successfully connected to database")
	return db
}

func createTable(db *sql.DB) {
	sqlStmt := fmt.Sprintf(`
		CREATE TABLE %s (
			id INT,
			name VARCHAR(255),
			est_diameter_min DOUBLE,
			est_diameter_max DOUBLE,
			relative_velocity DOUBLE,
			miss_distance DOUBLE,
			orbiting_body VARCHAR(255),
			sentry_object BOOLEAN,
			absolute_magnitude FLOAT,
			hazardous BOOLEAN
		);
	`, tableName)
	_, err := db.Exec(sqlStmt)
	if err != nil {
		panic(err)
	}
}

func dropTable(db *sql.DB) {
	sqlStmt := fmt.Sprintf("DROP TABLE %s", tableName)
	db.Exec(sqlStmt)
}

func exposePrometheusServer() {
	fmt.Printf("Exposing prometheus server\n")
	http.Handle("/metrics", promhttp.Handler())
	_ = http.ListenAndServe(":2112", nil)
	// if err != nil {
	// 	fmt.Printf("ERROR: Failed to serve a prometheus server: %s\n", err)
	// }
	fmt.Printf("Successfully exposed prometheus server\n")
}

func updateNObjectsFromDb(db *sql.DB, elements []*DataObject) {
	randNumber := rand.Float64() * 100
	stmt := fmt.Sprintf("UPDATE %s SET relative_velocity = %f", tableName, randNumber)
	_, err := db.Exec(stmt)
	if err != nil {
		panic(err)
	}
}

func getSystemMetricsForOperation(numberOfElements int, operation string, ticker *time.Ticker) {
	go func() {
		fmt.Printf("Starting proffiling for operation %s\n", operation)
		for range ticker.C {
			result, _ := cpu.Percent(0, false)
			systemMetrics[CPU_METRIC].With(
				prometheus.Labels{
					"replicas":  databaseReplicas,
					"operation": operation,
				},
			).Set(result[0])
			memStat, _ := mem.VirtualMemory()
			systemMetrics[MEM_METRIC].With(
				prometheus.Labels{
					"replicas":  databaseReplicas,
					"operation": operation,
				},
			).Set(memStat.UsedPercent)
		}
	}()
}

func getMetrics(db *sql.DB, objs []*DataObject, numberOfElements int) {
	fmt.Printf("Starting metrics for INSERT with %d objects\n", numberOfElements)
	insertTicker := time.NewTicker(defaultTickerDuration)
	cleanDatabase(db)
	getSystemMetricsForOperation(numberOfElements, INSERT_METRIC, insertTicker)
	begin := time.Now()
	createObjects(db, objs[:numberOfElements])
	end := time.Now()
	insertTicker.Stop()

	dataBaseMetrics[INSERT_METRIC].With(prometheus.Labels{
		"elements": fmt.Sprintf("%d", numberOfElements),
		"replicas": databaseReplicas,
	}).Set(end.Sub(begin).Seconds())

	fmt.Printf("Starting metrics for UPDATE with %d objects\n", numberOfElements)
	updateTicker := time.NewTicker(defaultTickerDuration)
	getSystemMetricsForOperation(numberOfElements, UPDATE_METRIC, updateTicker)
	begin = time.Now()
	updateNObjectsFromDb(db, objs[:numberOfElements])
	end = time.Now()
	updateTicker.Stop()

	dataBaseMetrics[UPDATE_METRIC].With(prometheus.Labels{
		"elements": fmt.Sprintf("%d", numberOfElements),
		"replicas": databaseReplicas,
	}).Set(end.Sub(begin).Seconds())

	fmt.Printf("Starting metrics for READ with %d objects\n", numberOfElements)
	readTicker := time.NewTicker(defaultTickerDuration)
	getSystemMetricsForOperation(numberOfElements, READ_METRIC, readTicker)
	begin = time.Now()
	readNObjectsFromDB(db, numberOfElements)
	end = time.Now()
	time.Sleep(time.Second * 5)
	readTicker.Stop()

	dataBaseMetrics[READ_METRIC].With(
		prometheus.Labels{
			"elements": fmt.Sprintf("%d", numberOfElements),
			"replicas": databaseReplicas,
		},
	).Set(end.Sub(begin).Seconds())

	fmt.Printf("Starting metrics for DELETE with %d objects\n", numberOfElements)
	deleteTicker := time.NewTicker(defaultTickerDuration)
	getSystemMetricsForOperation(numberOfElements, DELETE_METRIC, deleteTicker)
	begin = time.Now()
	deleteNObjectsFromDb(db, objs[:numberOfElements])
	end = time.Now()
	time.Sleep(time.Second * 5)
	deleteTicker.Stop()

	dataBaseMetrics[DELETE_METRIC].With(prometheus.Labels{
		"elements": fmt.Sprintf("%d", numberOfElements),
		"replicas": databaseReplicas,
	}).Set(end.Sub(begin).Seconds())
}

func deleteNObjectsFromDb(db *sql.DB, objs []*DataObject) {
	// tx, err := db.Begin()
	// if err != nil {
	// 	panic(err)
	// }
	// defer tx.Rollback()

	sqlStmt := fmt.Sprintf("DELETE FROM %s", tableName)

	db.Exec(sqlStmt)

	// stmt, err := tx.Prepare(sqlStmt)
	// if err != nil {
	// 	panic(err)
	// }
	// defer stmt.Close()

	// for _, obj := range objs {
	// 	stmt.Exec(obj.Id)
	// }

	// err = tx.Commit()
	// if err != nil {
	// 	panic(err)
	// }

}

func cleanDatabase(db *sql.DB) {
	fmt.Printf("Cleaning database\n")
	dropTable(db)
	createTable(db)
	fmt.Printf("Successfully cleaned database\n")
}

func main() {
	db := connectToDb()
	defer db.Close()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		exposePrometheusServer()
		wg.Done()
	}()

	cleanDatabase(db)
	fmt.Printf("Getting objects from csv\n")
	objs := getObjectsFromCsv()
	fmt.Printf("Successfully get objects from csv\n")

	totalElementsOnCsv := len(objs)

	firstBatch := int(totalElementsOnCsv / 3)    // 33% of elements in database
	secondBatch := int(totalElementsOnCsv/3) * 2 // 66% of elements in database
	thirdBatch := totalElementsOnCsv             // 100% of database

	iterations := 0
	for iterations < 30 {
		fmt.Printf("Starting metrics for iteration %d\n", iterations)
		getMetrics(db, objs, firstBatch)
		getMetrics(db, objs, secondBatch)
		getMetrics(db, objs, thirdBatch)
		iterations++
	}

	wg.Wait()
}
