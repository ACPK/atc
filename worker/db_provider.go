package worker

import (
	"fmt"
	"net"
	"time"

	gclient "github.com/cloudfoundry-incubator/garden/client"
	gconn "github.com/cloudfoundry-incubator/garden/client/connection"
	"github.com/pivotal-golang/clock"
	"github.com/pivotal-golang/lager"

	"github.com/concourse/atc/db"
)

//go:generate counterfeiter . WorkerDB

type WorkerDB interface {
	Workers() ([]db.WorkerInfo, error)
	// TODO: this is oddly named - it should be plural or return only one
	GetWorker(string) ([]db.WorkerInfo, bool, error)
	// Containers()
	CreateContainer(db.ContainerInfo) (db.ContainerInfo, bool, error)
	// UpdateContainer()
	GetContainer(string) (db.ContainerInfo, bool, error)
	FindContainerForIdentifier(db.ContainerIdentifier) (db.ContainerInfo, bool, error)
}

type dbProvider struct {
	logger lager.Logger
	db     WorkerDB
	dialer gconn.DialerFunc
}

func NewDBWorkerProvider(
	logger lager.Logger,
	db WorkerDB,
	dialer gconn.DialerFunc,
) WorkerProvider {
	return &dbProvider{
		logger: logger,
		db:     db,
		dialer: dialer,
	}
}

func (provider *dbProvider) Workers() ([]Worker, error) {
	workerInfos, err := provider.db.Workers()
	if err != nil {
		return nil, err
	}

	tikTok := clock.NewClock()

	workers := make([]Worker, len(workerInfos))
	for i, info := range workerInfos {
		// this is very important (to prevent closures capturing last value)
		addr := info.Addr

		workerLog := provider.logger.Session("worker-connection", lager.Data{
			"addr": addr,
		})

		connLog := workerLog.Session("garden-connection")

		var connection gconn.Connection

		if provider.dialer == nil {
			connection = gconn.NewWithLogger("tcp", addr, connLog)
		} else {
			dialer := func(string, string) (net.Conn, error) {
				return provider.dialer("tcp", addr)
			}

			connection = gconn.NewWithDialerAndLogger(dialer, connLog)
		}

		gardenConn := RetryableConnection{
			Logger:     workerLog,
			Connection: connection,
			Sleeper:    tikTok,
			RetryPolicy: ExponentialRetryPolicy{
				Timeout: 5 * time.Minute,
			},
		}

		workers[i] = NewGardenWorker(
			gclient.New(gardenConn),
			tikTok,
			info.ActiveContainers,
			info.ResourceTypes,
			info.Platform,
			info.Tags,
			info.Addr,
		)
	}

	return workers, nil
}

func (provider *dbProvider) GetWorker(name string) (Worker, bool, error) {
	workerInfos, success, err := provider.db.GetWorker(name)
	if err != nil {
		return nil, success, err
	}

	if len(workerInfos) == 0 {
		return nil, false, fmt.Errorf("No worker found for workerName: %s", name)
	}

	workerInfo := workerInfos[0]
	if len(workerInfos) > 1 {
		// TODO not sure what to return here
		return nil, success, nil
	}

	tikTok := clock.NewClock()
	workerLog := provider.logger.Session("worker-connection", lager.Data{
		"addr": workerInfo.Addr,
	})

	connLog := workerLog.Session("garden-connection")

	var connection gconn.Connection

	if provider.dialer == nil {
		connection = gconn.NewWithLogger("tcp", workerInfo.Addr, connLog)
	} else {
		dialer := func(string, string) (net.Conn, error) {
			return provider.dialer("tcp", workerInfo.Addr)
		}

		connection = gconn.NewWithDialerAndLogger(dialer, connLog)
	}

	gardenConn := RetryableConnection{
		Logger:     workerLog,
		Connection: connection,
		Sleeper:    tikTok,
		RetryPolicy: ExponentialRetryPolicy{
			Timeout: 5 * time.Minute,
		},
	}

	worker := NewGardenWorker(
		gclient.New(gardenConn),
		tikTok,
		workerInfo.ActiveContainers,
		workerInfo.ResourceTypes,
		workerInfo.Platform,
		workerInfo.Tags,
		workerInfo.Addr,
	)

	return worker, success, nil
}

func (provider *dbProvider) CreateContainer(containerInfo db.ContainerInfo) (db.ContainerInfo, bool, error) {
	return provider.db.CreateContainer(containerInfo)
}

func (provider *dbProvider) FindContainerForIdentifier(id Identifier) (db.ContainerInfo, bool, error) {
	containerIdentifier := db.ContainerIdentifier{
		Name:         id.Name,
		PipelineName: id.PipelineName,
		BuildID:      id.BuildID,
		Type:         id.Type,
		WorkerName:   id.WorkerName,
	}
	return provider.db.FindContainerForIdentifier(containerIdentifier)
}

func (provider *dbProvider) GetContainer(handle string) (db.ContainerInfo, bool, error) {
	return provider.db.GetContainer(handle)
}
