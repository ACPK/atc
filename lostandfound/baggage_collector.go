package lostandfound

import (
	"os"
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
	interval     time.Duration
}

func (bc *BaggageCollector) Run(logger lager.Logger) ifrit.Runner {
	return ifrit.RunFunc(func(signals <-chan os.Signal, ready chan<- struct{}) error {
		close(ready)

		for {
			timer := time.NewTimer(bc.interval)

			select {
			case <-signals:
				return nil
			case <-timer.C:

				leaseLogger := logger.Session("lease-invalidate-cache")

				lease, leased, err := bc.db.LeaseCacheInvalidation(bc.interval)

				if err != nil {
					leaseLogger.Error("failed-to-get-lease", err)
					break
				}

				if !leased {
					leaseLogger.Debug("did-not-get-lease")
					break
				}

				lease.Break()

			}
		}
	})
}

func NewBaggageCollector(workerClient worker.Client, db BaggageCollectorDB, interval time.Duration) *BaggageCollector {
	return &BaggageCollector{
		workerClient: workerClient,
		db:           db,
		interval:     interval,
	}
}
