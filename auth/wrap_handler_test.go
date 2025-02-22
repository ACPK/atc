package auth_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/auth/fakes"
)

var _ = Describe("WrapHandler", func() {
	var (
		fakeValidator *fakes.FakeValidator
		fakeRejector  *fakes.FakeRejector

		server *httptest.Server
		client *http.Client

		authenticated <-chan bool
	)

	BeforeEach(func() {
		fakeValidator = new(fakes.FakeValidator)
		fakeRejector = new(fakes.FakeRejector)

		fakeRejector.UnauthorizedStub = func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "nope", http.StatusUnauthorized)
		}

		a := make(chan bool, 1)
		authenticated = a
		simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			a <- auth.IsAuthenticated(r)
		})

		server = httptest.NewServer(auth.WrapHandler(
			simpleHandler,
			fakeValidator,
		))

		client = &http.Client{
			Transport: &http.Transport{},
		}
	})

	Context("when a request is made", func() {
		var request *http.Request
		var response *http.Response

		BeforeEach(func() {
			var err error

			request, err = http.NewRequest("GET", server.URL, bytes.NewBufferString("hello"))
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error

			response, err = client.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("when the validator returns true", func() {
			BeforeEach(func() {
				fakeValidator.IsAuthenticatedReturns(true)
			})

			It("handles the request with the request authenticated", func() {
				Expect(<-authenticated).To(BeTrue())
			})
		})

		Context("when the validator returns false", func() {
			BeforeEach(func() {
				fakeValidator.IsAuthenticatedReturns(false)
			})

			It("handles the request with the request authenticated", func() {
				Expect(<-authenticated).To(BeFalse())
			})
		})
	})
})
