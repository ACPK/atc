package api_test

import (
	"errors"
	"io/ioutil"
	"net/http"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	dbfakes "github.com/concourse/atc/db/fakes"
)

var _ = Describe("Resources API", func() {
	var pipelineDB *dbfakes.FakePipelineDB

	BeforeEach(func() {
		pipelineDB = new(dbfakes.FakePipelineDB)
		pipelineDBFactory.BuildWithNameReturns(pipelineDB, nil)
	})

	Describe("GET /api/v1/pipelines/:pipeline_name/resources", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			response, err = client.Get(server.URL + "/api/v1/pipelines/a-pipeline/resources")
			Expect(err).NotTo(HaveOccurred())

			Expect(pipelineDBFactory.BuildWithNameCallCount()).To(Equal(1))
			pipelineName := pipelineDBFactory.BuildWithNameArgsForCall(0)
			Expect(pipelineName).To(Equal("a-pipeline"))
		})

		Context("when getting the resource config succeeds", func() {
			BeforeEach(func() {
				pipelineDB.GetConfigReturns(atc.Config{
					Groups: []atc.GroupConfig{
						{
							Name:      "group-1",
							Resources: []string{"resource-1"},
						},
						{
							Name:      "group-2",
							Resources: []string{"resource-1", "resource-2"},
						},
					},

					Resources: []atc.ResourceConfig{
						{Name: "resource-1", Type: "type-1"},
						{Name: "resource-2", Type: "type-2"},
						{Name: "resource-3", Type: "type-3"},
					},
				}, 1, true, nil)
			})

			Context("when getting the check error succeeds", func() {
				BeforeEach(func() {
					pipelineDB.GetResourceStub = func(name string) (db.SavedResource, error) {
						if name == "resource-2" {
							return db.SavedResource{
								ID:           1,
								CheckError:   errors.New("sup"),
								PipelineName: "a-pipeline",
								Resource: db.Resource{
									Name: name,
								},
							}, nil
						} else {
							return db.SavedResource{
								ID:           2,
								Paused:       true,
								PipelineName: "a-pipeline",
								Resource: db.Resource{
									Name: name,
								},
							}, nil
						}
					}
				})

				It("returns 200 OK", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})

				Context("when authenticated", func() {
					BeforeEach(func() {
						authValidator.IsAuthenticatedReturns(true)
					})

					It("returns each resource, including their check failure", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
							{
								"name": "resource-1",
								"type": "type-1",
								"groups": ["group-1", "group-2"],
								"paused": true,
								"url": "/pipelines/a-pipeline/resources/resource-1"
							},
							{
								"name": "resource-2",
								"type": "type-2",
								"groups": ["group-2"],
								"url": "/pipelines/a-pipeline/resources/resource-2",
								"failing_to_check": true,
								"check_error": "sup"
							},
							{
								"name": "resource-3",
								"type": "type-3",
								"groups": [],
								"paused": true,
								"url": "/pipelines/a-pipeline/resources/resource-3"
							}
						]`))
					})
				})

				Context("when not authenticated", func() {
					BeforeEach(func() {
						authValidator.IsAuthenticatedReturns(false)
					})

					It("returns each resource, excluding their check failure", func() {
						body, err := ioutil.ReadAll(response.Body)
						Expect(err).NotTo(HaveOccurred())

						Expect(body).To(MatchJSON(`[
							{
								"name": "resource-1",
								"type": "type-1",
								"groups": ["group-1", "group-2"],
								"paused": true,
								"url": "/pipelines/a-pipeline/resources/resource-1"
							},
							{
								"name": "resource-2",
								"type": "type-2",
								"groups": ["group-2"],
								"url": "/pipelines/a-pipeline/resources/resource-2",
								"failing_to_check": true
							},
							{
								"name": "resource-3",
								"type": "type-3",
								"groups": [],
								"paused": true,
								"url": "/pipelines/a-pipeline/resources/resource-3"
							}
						]`))

					})
				})
			})

			Context("when getting the resource check error", func() {
				BeforeEach(func() {
					pipelineDB.GetResourceStub = func(name string) (db.SavedResource, error) {
						return db.SavedResource{}, errors.New("oh no!")
					}
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when getting the resource config fails", func() {
			Context("when the pipeline is no longer configured", func() {
				BeforeEach(func() {
					pipelineDB.GetConfigReturns(atc.Config{}, 0, false, nil)
				})

				It("returns 404", func() {
					Expect(response.StatusCode).To(Equal(http.StatusNotFound))
				})
			})

			Context("with an unknown error", func() {
				BeforeEach(func() {
					pipelineDB.GetConfigReturns(atc.Config{}, 0, false, errors.New("oh no!"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})
	})

	Describe("PUT /api/v1/pipelines/:pipeline_name/resources/:resource_name/pause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/pipelines/a-pipeline/resources/resource-name/pause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the proper pipelineDB", func() {
				Expect(pipelineDBFactory.BuildWithNameCallCount()).To(Equal(1))
				pipelineName := pipelineDBFactory.BuildWithNameArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
			})

			Context("when pausing the resource succeeds", func() {
				BeforeEach(func() {
					pipelineDB.PauseResourceReturns(nil)
				})

				It("paused the right resource", func() {
					Expect(pipelineDB.PauseResourceArgsForCall(0)).To(Equal("resource-name"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when pausing the resource fails", func() {
				BeforeEach(func() {
					pipelineDB.PauseResourceReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})

	Describe("PUT /api/v1/pipelines/:pipeline_name/resources/:resource_name/unpause", func() {
		var response *http.Response

		JustBeforeEach(func() {
			var err error

			request, err := http.NewRequest("PUT", server.URL+"/api/v1/pipelines/a-pipeline/resources/resource-name/unpause", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(true)
			})

			It("injects the proper pipelineDB", func() {
				Expect(pipelineDBFactory.BuildWithNameCallCount()).To(Equal(1))
				pipelineName := pipelineDBFactory.BuildWithNameArgsForCall(0)
				Expect(pipelineName).To(Equal("a-pipeline"))
			})

			Context("when unpausing the resource succeeds", func() {
				BeforeEach(func() {
					pipelineDB.UnpauseResourceReturns(nil)
				})

				It("unpaused the right resource", func() {
					Expect(pipelineDB.UnpauseResourceArgsForCall(0)).To(Equal("resource-name"))
				})

				It("returns 200", func() {
					Expect(response.StatusCode).To(Equal(http.StatusOK))
				})
			})

			Context("when unpausing the resource fails", func() {
				BeforeEach(func() {
					pipelineDB.UnpauseResourceReturns(errors.New("welp"))
				})

				It("returns 500", func() {
					Expect(response.StatusCode).To(Equal(http.StatusInternalServerError))
				})
			})
		})

		Context("when not authenticated", func() {
			BeforeEach(func() {
				authValidator.IsAuthenticatedReturns(false)
			})

			It("returns Unauthorized", func() {
				Expect(response.StatusCode).To(Equal(http.StatusUnauthorized))
			})
		})
	})
})
