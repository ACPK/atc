package versionserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api/present"
	"github.com/concourse/atc/db"
)

func (s *Server) ListResourceVersions(pipelineDB db.PipelineDB) http.Handler {
	logger := s.logger.Session("list-resource-versions")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			err   error
			until int
			since int
			limit int
		)

		resourceName := r.FormValue(":resource_name")

		urlUntil := r.FormValue(atc.PaginationQueryUntil)
		until, _ = strconv.Atoi(urlUntil)

		urlSince := r.FormValue(atc.PaginationQuerySince)
		since, _ = strconv.Atoi(urlSince)

		urlLimit := r.FormValue(atc.PaginationQueryLimit)

		limit, _ = strconv.Atoi(urlLimit)
		if limit == 0 {
			limit = atc.PaginationAPIDefaultLimit
		}

		versions, pagination, err := pipelineDB.GetResourceVersions(resourceName, db.Page{Until: until, Since: since, Limit: limit})
		if err != nil {
			logger.Error("failed-to-get-resource-versions", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if pagination.Next != nil {
			s.addNextLink(w, pipelineDB.GetPipelineName(), resourceName, *pagination.Next)
		}

		if pagination.Previous != nil {
			s.addPreviousLink(w, pipelineDB.GetPipelineName(), resourceName, *pagination.Previous)
		}

		w.Header().Set("Content-Type", "application/json")

		w.WriteHeader(http.StatusOK)

		resourceVersions := make([]atc.VersionedResource, len(versions))
		for i := 0; i < len(versions); i++ {
			resourceVersions[i] = present.SavedVersionedResource(versions[i])
		}
		json.NewEncoder(w).Encode(resourceVersions)
	})
}

func (s *Server) addNextLink(w http.ResponseWriter, pipelineName string, resourceName string, page db.Page) {
	w.Header().Add("Link", fmt.Sprintf(
		`<%s/api/v1/pipelines/%s/resources/%s/versions?%s=%d&%s=%d>; rel="%s"`,
		s.externalURL,
		pipelineName,
		resourceName,
		atc.PaginationQuerySince,
		page.Since,
		atc.PaginationQueryLimit,
		page.Limit,
		atc.LinkRelNext,
	))
}

func (s *Server) addPreviousLink(w http.ResponseWriter, pipelineName string, resourceName string, page db.Page) {
	w.Header().Add("Link", fmt.Sprintf(
		`<%s/api/v1/pipelines/%s/resources/%s/versions?%s=%d&%s=%d>; rel="%s"`,
		s.externalURL,
		pipelineName,
		resourceName,
		atc.PaginationQueryUntil,
		page.Until,
		atc.PaginationQueryLimit,
		page.Limit,
		atc.LinkRelPrevious,
	))
}
