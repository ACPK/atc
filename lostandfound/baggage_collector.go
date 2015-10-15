package lostandfound

import (
	"encoding/json"
	"time"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/worker"

	"github.com/pivotal-golang/lager"
)

type BaggageCollectorDB interface {
	GetAllActivePipelines() ([]db.SavedPipeline, error)
	GetVolumesToExpire() ([]db.VolumeData, error)
}

//go:generate counterfeiter . BaggageCollector

type BaggageCollector interface {
	Collect() error
}

type baggageCollector struct {
	logger            lager.Logger
	workerClient      worker.Client
	db                BaggageCollectorDB
	pipelineDBFactory db.PipelineDBFactory
}

func (bc *baggageCollector) Collect() error {
	bc.logger = bc.logger.Session("baggage-collection")

	resourceHashVersions, err := bc.getResourceHashVersions()
	if err != nil {
		panic(err)
		return err
	}

	err = bc.expireVolumes(resourceHashVersions)
	if err != nil {
		panic(err)
		return err
	}
	return nil
}

type resourceHashVersion map[string]int

func (bc *baggageCollector) getResourceHashVersions() (resourceHashVersion, error) {
	logger := bc.logger.Session("ranking-resource-versions")
	resourceHash := resourceHashVersion{}

	pipelines, err := bc.db.GetAllActivePipelines()
	if err != nil {
		logger.Info("could-not-get-active-pipelines", lager.Data{"error": err.Error()})
		return nil, err
	}

	for _, pipeline := range pipelines {
		pipelineDB := bc.pipelineDBFactory.Build(pipeline)
		resources := pipeline.Config.Resources

		for _, resource := range resources {
			//TODO: versions, err := pipelineDB.GetRecentResourceVersions(resource.Name, 5)
			dbResource, err := pipelineDB.GetResource(resource.Name)
			if err != nil {
				return nil, err
			}
			maxID, err := pipelineDB.GetResourceHistoryMaxID(dbResource.ID)
			if err != nil {
				return nil, err
			}

			resourceVersions, _, err := pipelineDB.GetResourceHistoryCursor(resource.Name, maxID, false, 5)
			if err != nil {
				return nil, err
			}

			for i, resourceVersion := range resourceVersions {
				version, _ := json.Marshal(resourceVersion.VersionedResource.Version)
				hashKey := string(version) + resource.Hash()
				resourceHash[hashKey] = i
			}
		}
	}

	return resourceHash, nil
}

func (bc *baggageCollector) expireVolumes(resourceHashVersions resourceHashVersion) error {
	logger := bc.logger.Session("expiring-volumes")
	volumesToExpire, err := bc.db.GetVolumesToExpire()

	if err != nil {
		logger.Info("could-not-get-volume-data", lager.Data{"error": err.Error()})
		return err
	}

	for _, volumeToExpire := range volumesToExpire {
		version, _ := json.Marshal(volumeToExpire.ResourceVersion)
		hashKey := string(version) + volumeToExpire.ResourceHash

		if rank, ok := resourceHashVersions[hashKey]; ok {
			if rank == 0 {
				continue
			}
		}

		worker, err := bc.workerClient.GetWorker(volumeToExpire.WorkerName)
		if err != nil {
			logger.Info("could-not-locate-worker", lager.Data{"error": err.Error()})
			continue
		}

		baggageClaimClient, found := worker.VolumeManager()

		if !found {
			logger.Info("no-volume-manager-on-worker", lager.Data{"error": err.Error()})
			continue
		}

		volume, err := baggageClaimClient.LookupVolume(logger, volumeToExpire.Handle)
		if err != nil {
			logger.Info("could-not-locate-volume", lager.Data{"error": err.Error()})
			continue
		}

		err = volume.SetTTL(8 * time.Hour)
		if err != nil {
			logger.Info("failed-to-set-ttl", lager.Data{"error": err.Error()})
		}
	}

	return nil

}

func NewBaggageCollector(
	logger lager.Logger,
	workerClient worker.Client,
	db BaggageCollectorDB,
	pipelineDBFactory db.PipelineDBFactory,
) BaggageCollector {
	return &baggageCollector{
		logger:            logger,
		workerClient:      workerClient,
		db:                db,
		pipelineDBFactory: pipelineDBFactory,
	}
}
