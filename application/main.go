package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gocarina/gocsv"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	datasetPath      = "./dataset/neo.csv"
	databasePassword = os.Getenv("MYSQL_ROOT_PASSWORD")
	databaseAddrs    = os.Getenv("MYSQL_SERVICE_ADDRS")
	databaseName     = "my_database" // This is the default database created when we deploy the mySql
	tableName        = "nearest_objects"
)

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

func readNObjectsFromDB(db *sql.DB, N int) {
	sqlStmt := fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, N)

	_, err := db.Query(sqlStmt)
	if err != nil {
		panic(err)
	}
}

func exposePrometheusServer() {
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}

func getInsertMetricsFor(db *sql.DB, objs []*DataObject, numberOfElements int) {
	cleanDatabase(db)
	begin := time.Now()
	createObjects(db, objs[:numberOfElements])
	end := time.Now()

	dataBaseMetrics[INSERT_METRIC].With(prometheus.Labels{"elements": fmt.Sprintf("%d", numberOfElements)}).Set(end.Sub(begin).Seconds())
}

func getReadMetricsFor(db *sql.DB, numberOfElements int) {
	begin := time.Now()
	readNObjectsFromDB(db, numberOfElements)
	end := time.Now()

	dataBaseMetrics[READ_METRIC].With(prometheus.Labels{"elements": fmt.Sprintf("%d", numberOfElements)}).Set(end.Sub(begin).Seconds())
}

func cleanDatabase(db *sql.DB) {
	dropTable(db)
	createTable(db)
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
	objs := getObjectsFromCsv()

	totalElementsOnCsv := len(objs)

	firstBatch := int(totalElementsOnCsv / 3)    // 33% of elements in database
	secondBatch := int(totalElementsOnCsv/3) * 2 // 66% of elements in database
	thirdBatch := totalElementsOnCsv             // 100% of database

	getInsertMetricsFor(db, objs, firstBatch)
	getInsertMetricsFor(db, objs, secondBatch)
	getInsertMetricsFor(db, objs, thirdBatch)

	// Here we have the 100% of elements inserted on database
	// So we can perform the read operation just limiting the
	// amount of elements returneds
	getReadMetricsFor(db, firstBatch)
	getReadMetricsFor(db, secondBatch)
	getReadMetricsFor(db, thirdBatch)

	wg.Wait()
}
