package worker_test

import (
	"errors"

	"github.com/cloudfoundry-incubator/garden"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/worker"
	"github.com/concourse/atc/worker/fakes"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Pool", func() {
	var (
		fakeProvider *fakes.FakeWorkerProvider

		pool   Client
		logger *lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeProvider = new(fakes.FakeWorkerProvider)
		logger = lagertest.NewTestLogger("test")

		pool = NewPool(fakeProvider, logger)
	})

	Describe("Create", func() {
		var (
			id   Identifier
			spec ContainerSpec

			createdContainer Container
			found            bool
			createErr        error
		)

		BeforeEach(func() {
			id = Identifier{Name: "some-name"}
			spec = ResourceTypeContainerSpec{Type: "some-type"}
		})

		JustBeforeEach(func() {
			createdContainer, found, createErr = pool.CreateContainer(id, spec)
		})

		Context("with multiple workers", func() {
			var (
				workerA *fakes.FakeWorker
				workerB *fakes.FakeWorker
				workerC *fakes.FakeWorker

				fakeContainer *fakes.FakeContainer
			)

			BeforeEach(func() {
				workerA = new(fakes.FakeWorker)
				workerB = new(fakes.FakeWorker)
				workerC = new(fakes.FakeWorker)

				workerA.ActiveContainersReturns(3)
				workerB.ActiveContainersReturns(2)

				workerA.SatisfiesReturns(true)
				workerB.SatisfiesReturns(true)

				fakeContainer = new(fakes.FakeContainer)
				workerA.CreateContainerReturns(fakeContainer, true, nil)
				workerB.CreateContainerReturns(fakeContainer, true, nil)
				workerC.CreateContainerReturns(fakeContainer, true, nil)

				fakeProvider.WorkersReturns([]Worker{workerA, workerB, workerC}, nil)
				fakeProvider.CreateContainerReturns(db.ContainerInfo{}, true, nil)
			})

			It("succeeds", func() {
				Ω(createErr).ShouldNot(HaveOccurred())
				Ω(found).Should(BeTrue())
			})

			It("returns the created container", func() {
				Ω(createdContainer).Should(Equal(fakeContainer))
			})

			It("checks that the workers satisfy the given spec", func() {
				Ω(workerA.SatisfiesCallCount()).Should(Equal(1))
				Ω(workerA.SatisfiesArgsForCall(0)).Should(Equal(spec))

				Ω(workerB.SatisfiesCallCount()).Should(Equal(1))
				Ω(workerB.SatisfiesArgsForCall(0)).Should(Equal(spec))

				Ω(workerC.SatisfiesCallCount()).Should(Equal(1))
				Ω(workerC.SatisfiesArgsForCall(0)).Should(Equal(spec))
			})

			It("creates using a random worker", func() {
				for i := 1; i < 100; i++ { // account for initial create in JustBefore
					createdContainer, found, createErr := pool.CreateContainer(id, spec)
					Ω(createErr).ShouldNot(HaveOccurred())
					Ω(found).Should(BeTrue())
					Ω(createdContainer).Should(Equal(fakeContainer))
				}

				Ω(workerA.CreateContainerCallCount()).Should(BeNumerically("~", workerB.CreateContainerCallCount(), 50))
				Ω(workerC.CreateContainerCallCount()).Should(BeZero())
			})

			Context("when creating the container fails", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					workerA.CreateContainerReturns(nil, false, disaster)
					workerB.CreateContainerReturns(nil, false, disaster)
				})

				It("returns the error", func() {
					Ω(createErr).Should(Equal(disaster))
				})
			})

			Context("when no workers satisfy the spec", func() {
				BeforeEach(func() {
					workerA.SatisfiesReturns(false)
					workerB.SatisfiesReturns(false)
					workerC.SatisfiesReturns(false)
				})

				It("returns a NoCompatibleWorkersError", func() {
					Ω(createErr).Should(Equal(NoCompatibleWorkersError{
						Spec:    spec,
						Workers: []Worker{workerA, workerB, workerC},
					}))
				})
			})
		})

		Context("with no workers", func() {
			BeforeEach(func() {
				fakeProvider.WorkersReturns([]Worker{}, nil)
			})

			It("returns ErrNoWorkers", func() {
				Ω(createErr).Should(Equal(ErrNoWorkers))
			})
		})

		Context("when getting the workers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeProvider.WorkersReturns(nil, disaster)
			})

			It("returns the error", func() {
				Ω(createErr).Should(Equal(disaster))
			})
		})
	})

	Describe("LookupContainer", func() {
		var (
			handle string
		)

		Context("with worker not in database", func() {
			BeforeEach(func() {
				fakeProvider.GetContainerReturns(db.ContainerInfo{}, true, nil)
				fakeProvider.GetWorkerReturns(nil, false, nil)
			})

			It("returns ErrDBGardenMismatch", func() {
				_, _, err := pool.LookupContainer(handle)

				Ω(err).Should(Equal(db.ErrDBGardenMismatch))
			})
		})

		Context("when getting the workers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeProvider.GetContainerReturns(db.ContainerInfo{}, true, nil)
				fakeProvider.GetWorkerReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				_, _, err := pool.LookupContainer(handle)

				Ω(err).Should(Equal(disaster))
			})
		})

		Context("with multiple workers", func() {
			var (
				workerA *fakes.FakeWorker
				workerB *fakes.FakeWorker

				fakeContainer *fakes.FakeContainer
			)

			BeforeEach(func() {
				workerA = new(fakes.FakeWorker)
				workerB = new(fakes.FakeWorker)

				// TODO: why do we need to set these?
				workerA.ActiveContainersReturns(3)
				workerB.ActiveContainersReturns(2)

				fakeContainer = new(fakes.FakeContainer)
				fakeContainer.HandleReturns("fake-container")

				fakeProvider.WorkersReturns([]Worker{workerA, workerB}, nil)
			})

			Context("when a worker can locate the container", func() {
				BeforeEach(func() {
					workerA.LookupContainerReturns(fakeContainer, true, nil)
					workerB.LookupContainerReturns(nil, false, garden.ContainerNotFoundError{})
				})

				It("returns the container", func() {
					foundContainer, found, err := pool.LookupContainer(handle)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(found).Should(BeTrue())

					Ω(foundContainer).Should(Equal(fakeContainer))
				})

				It("looks up by the given identifier", func() {
					_, _, err := pool.LookupContainer(handle)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(workerA.LookupContainerCallCount()).Should(Equal(1))
					Ω(workerB.LookupContainerCallCount()).Should(Equal(1))

					Ω(workerA.LookupContainerArgsForCall(0)).Should(Equal(handle))
					Ω(workerB.LookupContainerArgsForCall(0)).Should(Equal(handle))
				})
			})

			Context("when no workers can locate the container", func() {
				BeforeEach(func() {
					workerA.LookupContainerReturns(nil, false, garden.ContainerNotFoundError{})
					workerB.LookupContainerReturns(nil, false, garden.ContainerNotFoundError{})
				})

				It("returns ErrContainerNotFound", func() {
					_, _, err := pool.LookupContainer(handle)
					Ω(err).Should(Equal(garden.ContainerNotFoundError{}))
				})
			})
		})
	})

	Describe("FindContainerForIdentifier", func() {
		var (
			id Identifier

			foundContainer Container
			found          bool
			lookupErr      error
		)

		BeforeEach(func() {
			id = Identifier{Name: "some-name"}
		})

		JustBeforeEach(func() {
			foundContainer, found, lookupErr = pool.FindContainerForIdentifier(id)
		})

		Context("with multiple workers", func() {
			var (
				workerA *fakes.FakeWorker
				workerB *fakes.FakeWorker

				fakeContainer *fakes.FakeContainer
			)

			BeforeEach(func() {
				workerA = new(fakes.FakeWorker)
				workerB = new(fakes.FakeWorker)

				workerA.ActiveContainersReturns(3)
				workerB.ActiveContainersReturns(2)

				fakeContainer = new(fakes.FakeContainer)
				fakeContainer.HandleReturns("fake-container")

				fakeProvider.WorkersReturns([]Worker{workerA, workerB}, nil)
			})

			Context("when a worker can locate the container", func() {
				BeforeEach(func() {
					workerA.FindContainerForIdentifierReturns(fakeContainer, true, nil)
					workerB.FindContainerForIdentifierReturns(nil, false, nil)
				})

				It("returns the container", func() {
					Ω(foundContainer).Should(Equal(fakeContainer))
				})

				It("looks up by the given identifier", func() {
					Ω(workerA.FindContainerForIdentifierCallCount()).Should(Equal(1))
					Ω(workerB.FindContainerForIdentifierCallCount()).Should(Equal(1))

					Ω(workerA.FindContainerForIdentifierArgsForCall(0)).Should(Equal(id))
					Ω(workerB.FindContainerForIdentifierArgsForCall(0)).Should(Equal(id))
				})
			})

			Context("when no workers can locate the container", func() {
				BeforeEach(func() {
					workerA.FindContainerForIdentifierReturns(nil, false, nil)
					workerB.FindContainerForIdentifierReturns(nil, false, nil)
				})

				It("returns ErrContainerNotFound", func() {
					Ω(found).Should(BeFalse())
				})
			})

			Context("when multiple workers can locate the container", func() {
				var secondFakeContainer *fakes.FakeContainer

				BeforeEach(func() {
					secondFakeContainer = new(fakes.FakeContainer)
					secondFakeContainer.HandleReturns("second-fake-container")

					workerA.FindContainerForIdentifierReturns(fakeContainer, true, nil)
					workerB.FindContainerForIdentifierReturns(secondFakeContainer, true, nil)
				})

				It("returns a MultipleContainersError", func() {
					Ω(lookupErr).Should(BeAssignableToTypeOf(MultipleContainersError{}))
					Ω(lookupErr.(MultipleContainersError).Handles).Should(ConsistOf("fake-container", "second-fake-container"))
				})

				It("releases all returned containers", func() {
					Ω(fakeContainer.ReleaseCallCount()).Should(Equal(1))
					Ω(secondFakeContainer.ReleaseCallCount()).Should(Equal(1))
				})
			})

			Context("when a worker locates multiple containers", func() {
				var multiErr = MultipleContainersError{[]string{"a", "b"}}

				BeforeEach(func() {
					workerA.FindContainerForIdentifierReturns(fakeContainer, true, nil)
					workerB.FindContainerForIdentifierReturns(nil, false, multiErr)
				})

				It("returns a MultipleContainersError including the other found container", func() {
					Ω(lookupErr).Should(BeAssignableToTypeOf(MultipleContainersError{}))
					Ω(lookupErr.(MultipleContainersError).Handles).Should(ConsistOf("a", "b", "fake-container"))
				})

				It("releases all returned containers", func() {
					Ω(fakeContainer.ReleaseCallCount()).Should(Equal(1))
				})
			})

			Context("when multiple workers locate multiple containers", func() {
				var multiErrA = MultipleContainersError{[]string{"a", "b"}}
				var multiErrB = MultipleContainersError{[]string{"c", "d"}}

				BeforeEach(func() {
					workerA.FindContainerForIdentifierReturns(nil, false, multiErrA)
					workerB.FindContainerForIdentifierReturns(nil, false, multiErrB)
				})

				It("returns a MultipleContainersError including all found containers", func() {
					Ω(lookupErr).Should(BeAssignableToTypeOf(MultipleContainersError{}))
					Ω(lookupErr.(MultipleContainersError).Handles).Should(ConsistOf("a", "b", "c", "d"))
				})
			})
		})

		Context("with no workers", func() {
			BeforeEach(func() {
				fakeProvider.WorkersReturns([]Worker{}, nil)
			})

			It("returns ErrNoWorkers", func() {
				Ω(lookupErr).Should(Equal(ErrNoWorkers))
			})
		})

		Context("when getting the workers fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeProvider.WorkersReturns(nil, disaster)
			})

			It("returns the error", func() {
				Ω(lookupErr).Should(Equal(disaster))
			})
		})
	})
	Describe("Name", func() {
		It("responds correctly", func() {
			Ω(pool.Name()).To(Equal("pool"))
		})
	})
})
