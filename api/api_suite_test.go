package api_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-golang/lager"
	"github.com/pivotal-golang/lager/lagertest"

	"github.com/concourse/atc"
	"github.com/concourse/atc/api"
	"github.com/concourse/atc/api/buildserver"
	buildfakes "github.com/concourse/atc/api/buildserver/fakes"
	containerserverfakes "github.com/concourse/atc/api/containerserver/fakes"
	jobserverfakes "github.com/concourse/atc/api/jobserver/fakes"
	pipeserverfakes "github.com/concourse/atc/api/pipes/fakes"
	volumeserverfakes "github.com/concourse/atc/api/volumeserver/fakes"
	workerserverfakes "github.com/concourse/atc/api/workerserver/fakes"
	"github.com/concourse/atc/auth"
	authfakes "github.com/concourse/atc/auth/fakes"
	dbfakes "github.com/concourse/atc/db/fakes"
	enginefakes "github.com/concourse/atc/engine/fakes"
	workerfakes "github.com/concourse/atc/worker/fakes"
	"github.com/concourse/atc/wrappa"
)

var (
	sink *lager.ReconfigurableSink

	externalURL = "https://example.com"

	authValidator        *authfakes.FakeValidator
	fakeTokenGenerator   *authfakes.FakeTokenGenerator
	authProviders        auth.Providers
	basicAuthEnabled     = true
	fakeEngine           *enginefakes.FakeEngine
	fakeWorkerClient     *workerfakes.FakeClient
	buildsDB             *buildfakes.FakeBuildsDB
	volumesDB            *volumeserverfakes.FakeVolumesDB
	configDB             *dbfakes.FakeConfigDB
	workerDB             *workerserverfakes.FakeWorkerDB
	containerDB          *containerserverfakes.FakeContainerDB
	pipeDB               *pipeserverfakes.FakePipeDB
	pipelineDBFactory    *dbfakes.FakePipelineDBFactory
	pipelinesDB          *dbfakes.FakePipelinesDB
	fakeSchedulerFactory *jobserverfakes.FakeSchedulerFactory
	configValidationErr  error
	peerAddr             string
	drain                chan struct{}
	cliDownloadsDir      string

	constructedEventHandler *fakeEventHandlerFactory

	server *httptest.Server
	client *http.Client
)

type fakeEventHandlerFactory struct {
	db      buildserver.BuildsDB
	buildID int

	lock sync.Mutex
}

func (f *fakeEventHandlerFactory) Construct(
	db buildserver.BuildsDB,
	buildID int,
) http.Handler {
	f.lock.Lock()
	f.db = db
	f.buildID = buildID
	f.lock.Unlock()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake event handler factory was here"))
	})
}

var _ = BeforeEach(func() {
	buildsDB = new(buildfakes.FakeBuildsDB)
	configDB = new(dbfakes.FakeConfigDB)
	pipelineDBFactory = new(dbfakes.FakePipelineDBFactory)
	workerDB = new(workerserverfakes.FakeWorkerDB)
	containerDB = new(containerserverfakes.FakeContainerDB)
	volumesDB = new(volumeserverfakes.FakeVolumesDB)
	pipeDB = new(pipeserverfakes.FakePipeDB)
	pipelinesDB = new(dbfakes.FakePipelinesDB)

	authValidator = new(authfakes.FakeValidator)

	fakeTokenGenerator = new(authfakes.FakeTokenGenerator)

	authProvider1 := new(authfakes.FakeProvider)
	authProvider1.DisplayNameReturns("OAuth Provider 1")

	authProvider2 := new(authfakes.FakeProvider)
	authProvider2.DisplayNameReturns("OAuth Provider 2")

	authProviders = auth.Providers{
		"oauth-provider-1": authProvider1,
		"oauth-provider-2": authProvider2,
	}

	configValidationErr = nil
	peerAddr = "127.0.0.1:1234"
	drain = make(chan struct{})

	fakeEngine = new(enginefakes.FakeEngine)
	fakeWorkerClient = new(workerfakes.FakeClient)

	fakeSchedulerFactory = new(jobserverfakes.FakeSchedulerFactory)

	var err error

	cliDownloadsDir, err = ioutil.TempDir("", "cli-downloads")
	Expect(err).NotTo(HaveOccurred())

	constructedEventHandler = &fakeEventHandlerFactory{}

	logger := lagertest.NewTestLogger("callbacks")

	sink = lager.NewReconfigurableSink(lager.NewWriterSink(GinkgoWriter, lager.DEBUG), lager.DEBUG)
	logger.RegisterSink(sink)

	handler, err := api.NewHandler(
		logger,

		externalURL,

		wrappa.NewAPIAuthWrappa(authValidator),

		fakeTokenGenerator,
		authProviders,
		basicAuthEnabled,

		pipelineDBFactory,
		configDB,

		buildsDB,
		workerDB,
		containerDB,
		volumesDB,
		pipeDB,
		pipelinesDB,

		func(atc.Config) error { return configValidationErr },
		peerAddr,
		constructedEventHandler.Construct,
		drain,

		fakeEngine,
		fakeWorkerClient,

		fakeSchedulerFactory,

		sink,

		cliDownloadsDir,
	)
	Expect(err).NotTo(HaveOccurred())

	server = httptest.NewServer(handler)

	client = &http.Client{
		Transport: &http.Transport{},
	}
})

var _ = AfterEach(func() {
	server.Close()
})

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "API Suite")
}
