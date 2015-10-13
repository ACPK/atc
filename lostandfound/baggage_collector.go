package lostandfound

import "github.com/concourse/atc/worker"

type BaggageCollectorDB interface {
}

//go:generate counterfeiter . BaggageCollector

type BaggageCollector interface {
	Collect() error
}

type baggageCollector struct {
	workerClient worker.Client
	db           BaggageCollectorDB
}

func (bc *baggageCollector) Collect() error {
	return nil
}

func NewBaggageCollector(workerClient worker.Client, db BaggageCollectorDB) BaggageCollector {
	return &baggageCollector{
		workerClient: workerClient,
		db:           db,
	}
}
