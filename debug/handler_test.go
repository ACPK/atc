package debug_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/debug"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("VersionsDB Debug API", func() {
	var client *http.Client
	var server *httptest.Server

	BeforeEach(func() {
		handler, err := debug.NewHandler()
		Expect(err).NotTo(HaveOccurred())

		server = httptest.NewServer(handler)

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	Describe("GET /debug/versions-db-dump.json", func() {
		var response *http.Response

		BeforeEach(func() {
			pipelinesDB.GetPipelineByNameReturns(db.SavedPipeline{
				ID:     1,
				Paused: false,
				Pipeline: db.Pipeline{
					Name: "some-specific-pipeline",
				},
			}, nil)
		})

		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/debug/versions-db-dump.json", nil)
			Expect(err).NotTo(HaveOccurred())

			req.Header.Set("Content-Type", "application/json")

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200 OK", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("returns application/json", func() {
			Expect(response.Header.Get("Content-Type")).To(Equal("application/json"))
		})

	})
})
