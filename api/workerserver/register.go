package workerserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/metric"
	"github.com/pivotal-golang/lager"
)

type IntMetric int

func (i IntMetric) String() string {
	return strconv.Itoa(int(i))
}

func (s *Server) RegisterWorker(w http.ResponseWriter, r *http.Request) {
	var registration atc.Worker
	err := json.NewDecoder(r.Body).Decode(&registration)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(registration.GardenAddr) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "missing address")
		return
	}

	var ttl time.Duration

	ttlStr := r.URL.Query().Get("ttl")
	if len(ttlStr) > 0 {
		ttl, err = time.ParseDuration(ttlStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "malformed ttl")
			return
		}
	}

	metric.WorkerContainers{
		WorkerAddr: registration.GardenAddr,
		Containers: registration.ActiveContainers,
	}.Emit(s.logger)

	if registration.Name == "" {
		registration.Name = registration.Addr
	}

	err = s.db.SaveWorker(db.WorkerInfo{
		GardenAddr:       registration.GardenAddr,
		BaggageclaimURL:  registration.BaggageclaimURL,
		ActiveContainers: registration.ActiveContainers,
		ResourceTypes:    registration.ResourceTypes,
		Platform:         registration.Platform,
		Tags:             registration.Tags,
		Name:             registration.Name,
	}, ttl)
	if err != nil {
		s.logger.Info("Failed to register worker", lager.Data{"err": err.Error()})
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
