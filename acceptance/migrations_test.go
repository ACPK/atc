package acceptance_test

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/lib/pq"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-golang/lager/lagertest"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"
)

var _ = FDescribe("Migrations", func() {

	var (
		atcRunner  *ginkgomon.Runner
		atcProcess ifrit.Process
		dbListener *pq.Listener
		atcPort    uint16
	)

	BeforeEach(func() {
		logger := lagertest.NewTestLogger("test")
		postgresRunner.CreateTestDB()
		dbConn = postgresRunner.Open()
		dbListener = pq.NewListener(postgresRunner.DataSourceName(), time.Second, time.Minute, nil)
		bus := db.NewNotificationsBus(dbListener, dbConn)
		sqlDB = db.NewSQL(logger, dbConn, bus)

		_, err := sqlDB.SaveConfig(atc.DefaultPipelineName, atc.Config{}, db.ConfigVersion(1), db.PipelineUnpaused)
		Ω(err).ShouldNot(HaveOccurred())

		atcBin, err := gexec.Build("github.com/concourse/atc/cmd/atc")
		Ω(err).ShouldNot(HaveOccurred())

		atcPort = 5697 + uint16(GinkgoParallelNode())
		debugPort := 6697 + uint16(GinkgoParallelNode())

		atcCommand := exec.Command(
			atcBin,
			"-webListenPort", fmt.Sprintf("%d", atcPort),
			"-debugListenPort", fmt.Sprintf("%d", debugPort),
			"-httpUsername", "admin",
			"-httpPassword", "password",
			"-templates", filepath.Join("..", "web", "templates"),
			"-public", filepath.Join("..", "web", "public"),
			"-sqlDataSource", postgresRunner.DataSourceName(),
			"-skipMigrations",
		)
		atcRunner = ginkgomon.New(ginkgomon.Config{
			Command:       atcCommand,
			Name:          "atc",
			StartCheck:    "atc.listening",
			AnsiColorCode: "32m",
		})
		atcProcess = ginkgomon.Invoke(atcRunner)
	})

	AfterEach(func() {
		ginkgomon.Interrupt(atcProcess)

		Ω(dbConn.Close()).Should(Succeed())
		Ω(dbListener.Close()).Should(Succeed())

		postgresRunner.DropTestDB()
	})

	Context("when the -skipMigrations flag is provided", func() {
		FIt("Does not attempt to run the migration", func() {
			Ω(atcRunner).To(gbytes.Say("skipping migrations"))
		})
	})

	Context("when the -skipMigrations flag is omitted", func() {
		It("Attempts to run the migration", func() {
			Ω(atcRunner).To(gbytes.Say("running migrations"))
		})
	})
})
