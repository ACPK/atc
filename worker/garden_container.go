package worker

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc/metric"
	"github.com/pivotal-golang/clock"
)

type gardenWorkerContainer struct {
	garden.Container

	gardenClient garden.Client

	clock clock.Clock

	stopHeartbeating chan struct{}
	heartbeating     *sync.WaitGroup

	releaseOnce sync.Once

	identifier Identifier
}

func newGardenWorkerContainer(container garden.Container, gardenClient garden.Client, clock clock.Clock) (Container, error) {
	workerContainer := &gardenWorkerContainer{
		Container: container,

		gardenClient: gardenClient,

		clock: clock,

		heartbeating:     new(sync.WaitGroup),
		stopHeartbeating: make(chan struct{}),
	}

	workerContainer.heartbeating.Add(1)
	go workerContainer.heartbeat(clock.NewTicker(containerKeepalive))

	trackedContainers.Add(1)
	metric.TrackedContainers.Inc()

	err := workerContainer.initializeIdentifier()
	if err != nil {
		workerContainer.Release()
		return nil, err
	}
	return workerContainer, nil
}

func (container *gardenWorkerContainer) Destroy() error {
	container.Release()
	return container.gardenClient.Destroy(container.Handle())
}

func (container *gardenWorkerContainer) Release() {
	container.releaseOnce.Do(func() {
		close(container.stopHeartbeating)
		container.heartbeating.Wait()
		trackedContainers.Add(-1)
		metric.TrackedContainers.Dec()
	})
}

func (container *gardenWorkerContainer) initializeIdentifier() error {
	properties, err := container.Properties()
	if err != nil {
		fmt.Println("ERROR HERE")
		return err
	}

	propertyPrefix := "concourse:"
	identifier := Identifier{}

	nameKey := propertyPrefix + "name"
	if properties[nameKey] != "" {
		identifier.Name = properties[nameKey]
	}

	pipelineKey := propertyPrefix + "pipeline-name"
	if properties[pipelineKey] != "" {
		identifier.PipelineName = properties[pipelineKey]
	}

	buildIDKey := propertyPrefix + "build-id"
	if properties[buildIDKey] != "" {
		identifier.BuildID, err = strconv.Atoi(properties[buildIDKey])
		if err != nil {
			return err
		}
	}

	typeKey := propertyPrefix + "type"
	if properties[typeKey] != "" {
		identifier.Type = ContainerType(properties[typeKey])
	}

	stepLocationKey := propertyPrefix + "location"
	if properties[stepLocationKey] != "" {
		StepLocationUint, err := strconv.Atoi(properties[stepLocationKey])
		if err != nil {
			return err
		}
		identifier.StepLocation = uint(StepLocationUint)
	}

	checkTypeKey := propertyPrefix + "check-type"
	if properties[checkTypeKey] != "" {
		identifier.CheckType = properties[checkTypeKey]
	}

	checkSourceKey := propertyPrefix + "check-source"
	if properties[checkSourceKey] != "" {
		checkSourceString := properties[checkSourceKey]
		err := json.Unmarshal([]byte(checkSourceString), &identifier.CheckSource)
		if err != nil {
			return err
		}
	}

	container.identifier = identifier
	return nil
}

func (container *gardenWorkerContainer) IdentifierFromProperties() Identifier {
	return container.identifier
}

func (container *gardenWorkerContainer) heartbeat(pacemaker clock.Ticker) {
	defer container.heartbeating.Done()
	defer pacemaker.Stop()

	for {
		select {
		case <-pacemaker.C():
			container.SetProperty("keepalive", fmt.Sprintf("%d", container.clock.Now().Unix()))
		case <-container.stopHeartbeating:
			return
		}
	}
}
