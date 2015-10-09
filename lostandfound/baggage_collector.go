package lostandfound

import (
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
)

type BaggageCollectorDB interface {
	LeaseCacheInvalidation(interval time.Duration) (db.Lease, bool, error)
}

type BaggageCollector struct {
	workerClient worker.Client
	db           BaggageCollectorDB
}

func (bc *BaggageCollector) Run(logger lager.Logger, resourceName string) ifrit.Runner {
	return nil
}

func NewBaggageCollector(workerClient worker.Client, db BaggageCollectorDB) *BaggageCollector {
	return &BaggageCollector{
		workerClient: workerClient,
		db:           db,
	}
}
