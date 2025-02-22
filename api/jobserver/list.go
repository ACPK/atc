package jobserver

import (
	"encoding/json"
	"net/http"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListJobs(pipelineDB db.PipelineDB) http.Handler {
	logger := s.logger.Session("list-jobs")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var jobs []atc.Job
		config, _, found, err := pipelineDB.GetConfig()
		if err != nil {
			logger.Error("failed-to-get-config", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !found {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		for _, job := range config.Jobs {
			finished, next, err := pipelineDB.GetJobFinishedAndNextBuild(job.Name)
			if err != nil {
				logger.Error("failed-to-get-job-finished-and-next-build", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			dbJob, err := pipelineDB.GetJob(job.Name)
			if err != nil {
				logger.Error("failed-to-get-job", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			jobs = append(jobs, present.Job(dbJob, job, config.Groups, finished, next))
		}

		w.WriteHeader(http.StatusOK)

		json.NewEncoder(w).Encode(jobs)
	})
}
