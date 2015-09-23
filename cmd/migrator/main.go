package main

import (
	"flag"
	"os"

	Db "github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

var sqlDriver = flag.String(
	"sqlDriver",
	"postgres",
	"database/sql driver name",
)

var sqlDataSource = flag.String(
	"sqlDataSource",
	"postgres://127.0.0.1:5432/atc?sslmode=disable",
	"database/sql data source configuration string",
)

func main() {
	flag.Parse()

	logger := lager.NewLogger("migrator")

	logLevel := lager.INFO
	sink := lager.NewReconfigurableSink(lager.NewWriterSink(os.Stdout, lager.DEBUG), logLevel)
	logger.RegisterSink(sink)

	_, err := Db.RunMigrations(*sqlDriver, *sqlDataSource, logger)
	if err != nil {
		fatal(err)
	}
	logger.Info("running migrations completed successfully")
}

func fatal(err error) {
	println(err.Error())
	os.Exit(1)
}
