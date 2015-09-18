package worker

import (
	"errors"
	"expvar"
	"fmt"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc"
	"github.com/pivotal-golang/clock"
)

var ErrContainerNotFound = errors.New("container not found")
var ErrUnsupportedResourceType = errors.New("unsupported resource type")

const containerKeepalive = 30 * time.Second

const ephemeralPropertyName = "concourse:ephemeral"

var trackedContainers = expvar.NewInt("TrackedContainers")

//go:generate counterfeiter . Worker

// type Worker interface {
// Client

// ActiveContainers() int
// Satisfies(ContainerSpec) bool

// Description() string
// }

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

func (worker *gardenWorker) CreateContainer(id Identifier, spec ContainerSpec) (Container, error) {
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

		return nil, ErrUnsupportedResourceType

	case TaskContainerSpec:
		gardenSpec.RootFSPath = s.Image
		gardenSpec.Privileged = s.Privileged

	default:
		return nil, fmt.Errorf("unknown container spec type: %T (%#v)", s, s)
	}

	gardenContainer, err := worker.gardenClient.Create(gardenSpec)
	if err != nil {
		return nil, err
	}

	return newGardenWorkerContainer(gardenContainer, worker.gardenClient, worker.clock)
}

func (worker *gardenWorker) FindContainerForIdentifier(id Identifier) (Container, error) {
	containers, err := worker.gardenClient.Containers(id.gardenProperties())
	if err != nil {
		return nil, err
	}

	switch len(containers) {
	case 0:
		return nil, ErrContainerNotFound
	case 1:
		return newGardenWorkerContainer(containers[0], worker.gardenClient, worker.clock)
	default:
		handles := []string{}

		for _, c := range containers {
			handles = append(handles, c.Handle())
		}

		return nil, MultipleContainersError{
			Handles: handles,
		}
	}
}

func (worker *gardenWorker) FindContainersForIdentifier(id Identifier) ([]Container, error) {
	containers, err := worker.gardenClient.Containers(id.gardenProperties())
	if err != nil {
		return nil, err
	}

	gardenContainers := make([]Container, len(containers))
	for i, c := range containers {
		gardenContainers[i], err = newGardenWorkerContainer(c, worker.gardenClient, worker.clock)
		if err != nil {
			return nil, err
		}
	}

	return gardenContainers, nil
}

func (worker *gardenWorker) LookupContainer(handle string) (Container, error) {
	container, err := worker.gardenClient.Lookup(handle)
	if err != nil {
		return nil, err
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
