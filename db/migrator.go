package db

import (
	"database/sql"
	"strings"
	"time"

	"github.com/BurntSushi/migration"
	"github.com/concourse/atc/db/migrations"
	"github.com/pivotal-golang/lager"
)

func RunMigrations(driver string, dataSource string, logger lager.Logger) (*sql.DB, error) {
	logger.Info("running migrations")

	for {
		dbConn, err := migration.Open(driver, dataSource, migrations.Migrations)
		if err != nil {
			if strings.Contains(err.Error(), " dial ") {
				logger.Error("failed-to-open-db", err)
				time.Sleep(5 * time.Second)
				continue
			}
			return nil, err
		}

		return dbConn, nil
	}
}
