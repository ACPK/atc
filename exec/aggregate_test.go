package exec_test

import (
	"errors"
	"os"
	"sync"

	. "github.com/concourse/atc/exec"

	"github.com/concourse/atc/exec/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tedsuo/ifrit"
)

var _ = Describe("Aggregate", func() {
	var (
		fakeStepA *fakes.FakeStepFactory
		fakeStepB *fakes.FakeStepFactory

		aggregate StepFactory

		inStep *fakes.FakeStep
		repo   *SourceRepository

		outStepA *fakes.FakeStep
		outStepB *fakes.FakeStep

		step    Step
		process ifrit.Process
	)

	BeforeEach(func() {
		fakeStepA = new(fakes.FakeStepFactory)
		fakeStepB = new(fakes.FakeStepFactory)

		aggregate = Aggregate{
			fakeStepA,
			fakeStepB,
		}

		inStep = new(fakes.FakeStep)
		repo = NewSourceRepository()

		outStepA = new(fakes.FakeStep)
		fakeStepA.UsingReturns(outStepA)

		outStepB = new(fakes.FakeStep)
		fakeStepB.UsingReturns(outStepB)
	})

	JustBeforeEach(func() {
		step = aggregate.Using(inStep, repo)
		process = ifrit.Invoke(step)
	})

	It("uses the input source for all steps", func() {
		Expect(fakeStepA.UsingCallCount()).To(Equal(1))
		step, repo := fakeStepA.UsingArgsForCall(0)
		Expect(step).To(Equal(inStep))
		Expect(repo).To(Equal(repo))

		Expect(fakeStepB.UsingCallCount()).To(Equal(1))
		step, repo = fakeStepB.UsingArgsForCall(0)
		Expect(step).To(Equal(inStep))
		Expect(repo).To(Equal(repo))
	})

	It("exits successfully", func() {
		Eventually(process.Wait()).Should(Receive(BeNil()))
	})

	Describe("executing each source", func() {
		BeforeEach(func() {
			wg := new(sync.WaitGroup)
			wg.Add(2)

			outStepA.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				wg.Done()
				wg.Wait()
				close(ready)
				return nil
			}

			outStepB.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				wg.Done()
				wg.Wait()
				close(ready)
				return nil
			}
		})

		It("happens concurrently", func() {
			Expect(outStepA.RunCallCount()).To(Equal(1))
			Expect(outStepB.RunCallCount()).To(Equal(1))
		})
	})

	Describe("signalling", func() {
		var receivedSignals chan os.Signal

		BeforeEach(func() {
			receivedSignals = make(chan os.Signal, 2)

			outStepA.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				receivedSignals <- <-signals
				return ErrInterrupted
			}

			outStepB.RunStub = func(signals <-chan os.Signal, ready chan<- struct{}) error {
				close(ready)
				receivedSignals <- <-signals
				return ErrInterrupted
			}
		})

		It("propagates to all sources", func() {
			interruptedErr := errors.New("sources failed:\ninterrupted\ninterrupted")
			process.Signal(os.Interrupt)

			Eventually(process.Wait()).Should(Receive(Equal(interruptedErr)))
		})
	})

	Context("when sources fail", func() {
		disasterA := errors.New("nope A")
		disasterB := errors.New("nope B")

		BeforeEach(func() {
			outStepA.RunReturns(disasterA)
			outStepB.RunReturns(disasterB)
		})

		It("exits with an error including the original message", func() {
			var err error
			Eventually(process.Wait()).Should(Receive(&err))

			Expect(err.Error()).To(ContainSubstring("nope A"))
			Expect(err.Error()).To(ContainSubstring("nope B"))
		})
	})

	Describe("releasing", func() {
		It("releases all sources", func() {
			step.Release()

			Expect(outStepA.ReleaseCallCount()).To(Equal(1))
			Expect(outStepB.ReleaseCallCount()).To(Equal(1))
		})
	})

	Describe("getting a result", func() {
		Context("when the result type is bad", func() {
			It("returns false", func() {
				result := "this-is-bad"
				Expect(step.Result(&result)).To(BeFalse())
			})
		})

		Context("when getting a Success result", func() {
			var result Success

			BeforeEach(func() {
				result = false
			})

			Context("and all branches are successful", func() {
				BeforeEach(func() {
					outStepA.ResultStub = successResult(true)
					outStepB.ResultStub = successResult(true)
				})

				It("yields true", func() {
					Expect(step.Result(&result)).To(BeTrue())
					Expect(result).To(Equal(Success(true)))
				})
			})

			Context("and some branches are not successful", func() {
				BeforeEach(func() {
					outStepA.ResultStub = successResult(true)
					outStepB.ResultStub = successResult(false)
				})

				It("yields false", func() {
					Expect(step.Result(&result)).To(BeTrue())
					Expect(result).To(Equal(Success(false)))
				})
			})

			Context("when some branches do not indicate success", func() {
				BeforeEach(func() {
					outStepA.ResultStub = successResult(true)
					outStepB.ResultReturns(false)
				})

				It("only considers the branches that do", func() {
					Expect(step.Result(&result)).To(BeTrue())
					Expect(result).To(Equal(Success(true)))
				})
			})

			Context("when no branches indicate success", func() {
				BeforeEach(func() {
					outStepA.ResultReturns(false)
					outStepB.ResultReturns(false)
				})

				It("returns false", func() {
					Expect(step.Result(&result)).To(BeFalse())
					Expect(result).To(Equal(Success(false)))
				})
			})

			Context("when there are no branches", func() {
				BeforeEach(func() {
					aggregate = Aggregate{}
				})

				It("returns true", func() {
					Expect(step.Result(&result)).To(BeTrue())
					Expect(result).To(Equal(Success(true)))
				})
			})
		})
	})
})
