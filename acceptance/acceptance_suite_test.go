package acceptance_test

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/sclevine/agouti"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/postgresrunner"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/ginkgomon"

	"testing"
	"time"
)

func TestAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Acceptance Suite")
}

var atcBin string

var postgresRunner postgresrunner.Runner
var dbConn *sql.DB
var dbProcess ifrit.Process

var sqlDB *db.SQLDB

var agoutiDriver *agouti.WebDriver

var _ = SynchronizedBeforeSuite(func() []byte {
	atcBin, err := gexec.Build("github.com/concourse/atc/cmd/atc")
	Expect(err).NotTo(HaveOccurred())

	return []byte(atcBin)
}, func(b []byte) {
	atcBin = string(b)

	SetDefaultEventuallyTimeout(10 * time.Second)
	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)

	postgresRunner = postgresrunner.Runner{
		Port: 5432 + GinkgoParallelNode(),
	}

	dbProcess = ifrit.Invoke(postgresRunner)

	postgresRunner.CreateTestDB()

	agoutiDriver = agouti.PhantomJS()
	Expect(agoutiDriver.Start()).To(Succeed())
})

var _ = AfterSuite(func() {
	Expect(agoutiDriver.Stop()).To(Succeed())

	dbProcess.Signal(os.Interrupt)
	Eventually(dbProcess.Wait(), 10*time.Second).Should(Receive())
})

func Screenshot(page *agouti.Page) {
	page.Screenshot("/tmp/screenshot.png")
}

func Authenticate(page *agouti.Page, username, password string) {
	header := fmt.Sprintf("%s:%s", username, password)

	page.SetCookie(&http.Cookie{
		Name:  auth.CookieName,
		Value: "Basic " + base64.StdEncoding.EncodeToString([]byte(header)),
	})

	// PhantomJS won't send the cookie on ajax requests if the page is not
	// refreshed
	page.Refresh()
}

func startATC(atcBin string, atcServerNumber uint16) (ifrit.Process, uint16) {
	atcPort := 5697 + uint16(GinkgoParallelNode()) + (atcServerNumber * 100)
	debugPort := 6697 + uint16(GinkgoParallelNode()) + (atcServerNumber * 100)

	atcCommand := exec.Command(
		atcBin,
		"--bind-port", fmt.Sprintf("%d", atcPort),
		"--peer-url", fmt.Sprintf("http://127.0.0.1:%d", atcPort),
		"--postgres-data-source", postgresRunner.DataSourceName(),
		"--debug-bind-port", fmt.Sprintf("%d", debugPort),
		"--basic-auth-username", "admin",
		"--basic-auth-password", "password",
		"--publicly-viewable",
		"--templates", filepath.Join("..", "web", "templates"),
		"--public", filepath.Join("..", "web", "public"),
	)
	atcRunner := ginkgomon.New(ginkgomon.Config{
		Command:       atcCommand,
		Name:          "atc",
		StartCheck:    "atc.listening",
		AnsiColorCode: "32m",
	})

	return ginkgomon.Invoke(atcRunner), atcPort
}
