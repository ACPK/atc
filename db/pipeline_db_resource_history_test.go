package db_test

import (
	"database/sql"

	"fmt"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Resource History", func() {
	var dbConn *sql.DB
	var listener *pq.Listener

	var pipelineDBFactory db.PipelineDBFactory
	var sqlDB *db.SQLDB
	var pipelineDB db.PipelineDB
	var otherPipelineDB db.PipelineDB

	BeforeEach(func() {
		postgresRunner.Truncate()

		dbConn = postgresRunner.Open()

		listener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		Eventually(listener.Ping, 5*time.Second).ShouldNot(HaveOccurred())
		bus := db.NewNotificationsBus(listener, dbConn)

		sqlDB = db.NewSQL(lagertest.NewTestLogger("test"), dbConn, bus)
		pipelineDBFactory = db.NewPipelineDBFactory(lagertest.NewTestLogger("test"), dbConn, bus, sqlDB)

		_, err := sqlDB.SaveConfig("a-pipeline-name", atc.Config{}, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		pipelineDB, err = pipelineDBFactory.BuildWithName("a-pipeline-name")
		Expect(err).NotTo(HaveOccurred())

		_, err = sqlDB.SaveConfig("another-pipeline", atc.Config{}, 0, db.PipelineUnpaused)
		Expect(err).NotTo(HaveOccurred())

		otherPipelineDB, err = pipelineDBFactory.BuildWithName("another-pipeline")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := dbConn.Close()
		Expect(err).NotTo(HaveOccurred())

		err = listener.Close()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("GetResourceHistory", func() {
		var resource atc.ResourceConfig
		var versions []atc.Version
		var expectedHistories []*db.VersionHistory

		BeforeEach(func() {
			resource = atc.ResourceConfig{
				Name:   "some-resource",
				Type:   "some-type",
				Source: atc.Source{"some": "source"},
			}

			for i := 0; i < 10; i++ {
				version := atc.Version{"version": fmt.Sprintf("%d", i+1)}
				versions = append(versions, version)
				expectedHistories = append(expectedHistories,
					&db.VersionHistory{
						VersionedResource: db.SavedVersionedResource{
							ID:      i + 1,
							Enabled: true,
							VersionedResource: db.VersionedResource{
								Resource:     resource.Name,
								Type:         resource.Type,
								Version:      db.Version(version),
								Metadata:     []db.MetadataField{},
								PipelineName: pipelineDB.GetPipelineName(),
							},
						},
					})
			}

			err := pipelineDB.SaveResourceVersions(resource, versions)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when there are no versions to be found", func() {
			It("returns the versions, with previous/next pages", func() {
				historyPage, pagination, err := pipelineDB.GetResourceHistory("nope", db.Page{})
				Expect(err).ToNot(HaveOccurred())
				Expect(historyPage).To(Equal([]*db.VersionHistory{}))
				Expect(pagination).To(Equal(db.Pagination{}))
			})
		})

		Context("with no since/until", func() {
			// TODO: This test is failing because the test resource isn't saved in the database
			It("returns the first page, with the given limit, and a next page", func() {
				historyPage, pagination, err := pipelineDB.GetResourceHistory("job-name", db.Page{Limit: 2})
				Expect(err).ToNot(HaveOccurred())
				Expect(historyPage).To(Equal([]*db.VersionHistory{expectedHistories[9], expectedHistories[8]}))
				Expect(pagination.Previous).To(BeNil())
				Expect(pagination.Next).To(Equal(&db.Page{Since: expectedHistories[8].VersionedResource.ID, Limit: 2}))
			})
		})
		//
		// Context("with a since that places it in the middle of the builds", func() {
		// 	It("returns the builds, with previous/next pages", func() {
		// 		buildsPage, pagination, err := pipelineDB.GetJobBuilds("job-name", db.Page{Since: builds[6].ID, Limit: 2})
		// 		Expect(err).ToNot(HaveOccurred())
		// 		Expect(buildsPage).To(Equal([]db.Build{builds[5], builds[4]}))
		// 		Expect(pagination.Previous).To(Equal(&db.Page{Until: builds[5].ID, Limit: 2}))
		// 		Expect(pagination.Next).To(Equal(&db.Page{Since: builds[4].ID, Limit: 2}))
		// 	})
		// })
		//
		// Context("with a since that places it at the end of the builds", func() {
		// 	It("returns the builds, with previous/next pages", func() {
		// 		buildsPage, pagination, err := pipelineDB.GetJobBuilds("job-name", db.Page{Since: builds[2].ID, Limit: 2})
		// 		Expect(err).ToNot(HaveOccurred())
		// 		Expect(buildsPage).To(Equal([]db.Build{builds[1], builds[0]}))
		// 		Expect(pagination.Previous).To(Equal(&db.Page{Until: builds[1].ID, Limit: 2}))
		// 		Expect(pagination.Next).To(BeNil())
		// 	})
		// })
		//
		// Context("with an until that places it in the middle of the builds", func() {
		// 	It("returns the builds, with previous/next pages", func() {
		// 		buildsPage, pagination, err := pipelineDB.GetJobBuilds("job-name", db.Page{Until: builds[6].ID, Limit: 2})
		// 		Expect(err).ToNot(HaveOccurred())
		// 		Expect(buildsPage).To(Equal([]db.Build{builds[8], builds[7]}))
		// 		Expect(pagination.Previous).To(Equal(&db.Page{Until: builds[8].ID, Limit: 2}))
		// 		Expect(pagination.Next).To(Equal(&db.Page{Since: builds[7].ID, Limit: 2}))
		// 	})
		// })
		//
		// Context("with a until that places it at the beginning of the builds", func() {
		// 	It("returns the builds, with previous/next pages", func() {
		// 		buildsPage, pagination, err := pipelineDB.GetJobBuilds("job-name", db.Page{Until: builds[7].ID, Limit: 2})
		// 		Expect(err).ToNot(HaveOccurred())
		// 		Expect(buildsPage).To(Equal([]db.Build{builds[9], builds[8]}))
		// 		Expect(pagination.Previous).To(BeNil())
		// 		Expect(pagination.Next).To(Equal(&db.Page{Since: builds[8].ID, Limit: 2}))
		// 	})
		// })
	})
})
