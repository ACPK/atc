package worker

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/concourse/atc/db"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . WorkerProvider

type WorkerProvider interface {
	Workers() ([]Worker, error)
	GetWorker(string) (Worker, bool, error)
	CreateContainer(db.ContainerInfo) (db.ContainerInfo, bool, error)
	FindContainerForIdentifier(Identifier) (db.ContainerInfo, bool, error)
	GetContainer(string) (db.ContainerInfo, bool, error)
}

var ErrNoWorkers = errors.New("no workers")

type NoCompatibleWorkersError struct {
	Spec    ContainerSpec
	Workers []Worker
}

func (err NoCompatibleWorkersError) Error() string {
	availableWorkers := ""
	for _, worker := range err.Workers {
		availableWorkers += "\n  - " + worker.Description()
	}

	return fmt.Sprintf(
		"no workers satisfying: %s\n\navailable workers: %s",
		err.Spec.Description(),
		availableWorkers,
	)
}

type Pool struct {
	provider WorkerProvider
	logger   lager.Logger

	rand *rand.Rand
}

func NewPool(provider WorkerProvider, logger lager.Logger) Client {
	return &Pool{
		provider: provider,
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
		logger:   logger,
	}
}

func (pool *Pool) CreateContainer(id Identifier, spec ContainerSpec) (Container, bool, error) {
	workers, err := pool.provider.Workers()
	if err != nil {
		return nil, false, err
	}

	if len(workers) == 0 {
		return nil, false, ErrNoWorkers
	}

	compatibleWorkers := []Worker{}
	for _, worker := range workers {
		if worker.Satisfies(spec) {
			compatibleWorkers = append(compatibleWorkers, worker)
		}
	}

	if len(compatibleWorkers) == 0 {
		return nil, false, NoCompatibleWorkersError{
			Spec:    spec,
			Workers: workers,
		}
	}

	randomWorker := compatibleWorkers[pool.rand.Intn(len(compatibleWorkers))]

	container, found, err := randomWorker.CreateContainer(id, spec)
	if err != nil {
		return nil, found, err
	}

	containerInfo := db.ContainerInfo{
		Handle:       container.Handle(),
		Name:         id.Name,
		PipelineName: id.PipelineName,
		BuildID:      id.BuildID,
		Type:         id.Type,
		WorkerName:   randomWorker.Name(),
	}

	_, found, err = pool.provider.CreateContainer(containerInfo)
	if err != nil {
		// TODO: should we release the container here?
		// Any other cleanup to ensure the DB and real-life are in sync?
		return nil, found, err
	}
	return container, found, nil
}

func (pool *Pool) FindContainerForIdentifier(id Identifier) (Container, bool, error) {
	pool.logger.Info("finding container for identifier", lager.Data{"identifier": id})

	containerInfo, found, err := pool.provider.FindContainerForIdentifier(id)
	if err != nil {
		return nil, found, err
	}
	if !found {
		return nil, found, err
	}

	worker, found, err := pool.provider.GetWorker(containerInfo.WorkerName)
	if err != nil {
		return nil, found, err
	}

	container, found, err := worker.LookupContainer(containerInfo.Handle)
	if err != nil {
		return nil, found, err
	}

	return container, found, nil
}

type workerErrorInfo struct {
	workerName string
	err        error
}

type foundContainer struct {
	workerName string
	container  Container
}

func (pool *Pool) LookupContainer(handle string) (Container, bool, error) {
	pool.logger.Info("looking up container", lager.Data{"handle": handle})

	containerInfo, found, err := pool.provider.GetContainer(handle)
	if err != nil {
		return nil, false, err
	}
	if !found {
		return nil, false, nil
	}

	worker, found, err := pool.provider.GetWorker(containerInfo.WorkerName)
	if err != nil {
		return nil, false, err
	}
	// TODO: what does it mean to have a container belonging to a worker that
	// does not exist in the DB
	if !found {
		return nil, false, nil
	}

	container, found, err := worker.LookupContainer(handle)
	if err != nil {
		return nil, false, err
	}

	// TODO: what does it mean to have a container in the DB that garden does not
	// know about?
	if !found {
		return nil, false, nil
	}

	return container, found, nil
}

func (pool *Pool) Name() string {
	return "pool"
}

type byActiveContainers []Worker

func (cs byActiveContainers) Len() int { return len(cs) }

func (cs byActiveContainers) Swap(i, j int) { cs[i], cs[j] = cs[j], cs[i] }

func (cs byActiveContainers) Less(i, j int) bool {
	return cs[i].ActiveContainers() < cs[j].ActiveContainers()
}
