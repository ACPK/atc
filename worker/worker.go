package worker

import (
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
	"github.com/pivotal-golang/clock"
)

var ErrUnsupportedResourceType = errors.New("unsupported resource type")

const containerKeepalive = 30 * time.Second

const ephemeralPropertyName = "concourse:ephemeral"

var trackedContainers = expvar.NewInt("TrackedContainers")

//go:generate counterfeiter . Worker

type Worker interface {
	Client

	ActiveContainers() int
	Satisfies(ContainerSpec) bool

	Description() string
}

type gardenWorker struct {
	gardenClient garden.Client
	clock        clock.Clock

	activeContainers int
	resourceTypes    []atc.WorkerResourceType
	platform         string
	tags             []string
	name             string
}

func NewGardenWorker(
	gardenClient garden.Client,
	clock clock.Clock,
	activeContainers int,
	resourceTypes []atc.WorkerResourceType,
	platform string,
	tags []string,
	name string,
) Worker {
	return &gardenWorker{
		gardenClient: gardenClient,
		clock:        clock,

		activeContainers: activeContainers,
		resourceTypes:    resourceTypes,
		platform:         platform,
		tags:             tags,
		name:             name,
	}
}

func (worker *gardenWorker) CreateContainer(id Identifier, spec ContainerSpec) (Container, bool, error) {
	gardenSpec := garden.ContainerSpec{
		Properties: id.gardenProperties(),
	}

dance:
	switch s := spec.(type) {
	case ResourceTypeContainerSpec:
		gardenSpec.Privileged = true

		if s.Ephemeral {
			gardenSpec.Properties[ephemeralPropertyName] = "true"
		}

		for _, t := range worker.resourceTypes {
			if t.Type == s.Type {
				gardenSpec.RootFSPath = t.Image
				break dance
			}
		}

		return nil, false, ErrUnsupportedResourceType

	case TaskContainerSpec:
		gardenSpec.RootFSPath = s.Image
		gardenSpec.Privileged = s.Privileged

	default:
		return nil, false, fmt.Errorf("unknown container spec type: %T (%#v)", s, s)
	}

	gardenContainer, err := worker.gardenClient.Create(gardenSpec)
	if err != nil {
		return nil, false, err
	}

	return newGardenWorkerContainer(gardenContainer, worker.gardenClient, worker.clock)
}

func (worker *gardenWorker) FindContainerForIdentifier(id Identifier) (Container, bool, error) {
	containers, err := worker.gardenClient.Containers(id.gardenProperties())
	if err != nil {
		return nil, false, err
	}

	switch len(containers) {
	case 0:
		return nil, false, nil
	case 1:
		return newGardenWorkerContainer(containers[0], worker.gardenClient, worker.clock)
	default:
		handles := []string{}

		for _, c := range containers {
			handles = append(handles, c.Handle())
		}

		return nil, false, MultipleContainersError{
			Handles: handles,
		}
	}
}

func (worker *gardenWorker) LookupContainer(handle string) (Container, bool, error) {
	container, err := worker.gardenClient.Lookup(handle)
	if err != nil {
		return nil, false, err
	}

	if _, ok := err.(garden.ContainerNotFoundError); ok {
		return nil, false, nil
	}
	return newGardenWorkerContainer(container, worker.gardenClient, worker.clock)
}

func (worker *gardenWorker) ActiveContainers() int {
	return worker.activeContainers
}

func (worker *gardenWorker) Satisfies(spec ContainerSpec) bool {
	switch s := spec.(type) {
	case ResourceTypeContainerSpec:
		for _, t := range worker.resourceTypes {
			if t.Type == s.Type {
				return worker.tagsMatch(s.Tags)
			}
		}

		return false

	case TaskContainerSpec:
		if s.Platform != worker.platform {
			return false
		}

		return worker.tagsMatch(s.Tags)
	}

	return false
}

func (worker *gardenWorker) tagsMatch(tags []string) bool {
	if len(worker.tags) > 0 && len(tags) == 0 {
		return false
	}

insert_coin:
	for _, stag := range tags {
		for _, wtag := range worker.tags {
			if stag == wtag {
				continue insert_coin
			}
		}

		return false
	}

	return true
}

func (worker *gardenWorker) Description() string {
	messages := []string{
		fmt.Sprintf("platform '%s'", worker.platform),
	}

	for _, tag := range worker.tags {
		messages = append(messages, fmt.Sprintf("tag '%s'", tag))
	}

	return strings.Join(messages, ", ")
}

func (worker *gardenWorker) Name() string {
	return worker.name
}

type gardenWorkerContainer struct {
	garden.Container

	gardenClient garden.Client

	clock clock.Clock

	stopHeartbeating chan struct{}
	heartbeating     *sync.WaitGroup

	releaseOnce sync.Once

	identifier Identifier
}

func newGardenWorkerContainer(
	container garden.Container,
	gardenClient garden.Client,
	clock clock.Clock,
) (Container, bool, error) {
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
		return nil, false, err
	}
	return workerContainer, true, nil
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
		identifier.Type = db.ContainerType(properties[typeKey])
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
