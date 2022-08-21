package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"

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

func exposePrometheusServer() {

	dataBaseMetrics[0].With(prometheus.Labels{"elements": "30"}).Set(223.5)

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}

func main() {

	exposePrometheusServer()

}
