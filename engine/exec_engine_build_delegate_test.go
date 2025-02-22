package engine_test

import (
	"errors"
	"io"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	. "github.com/concourse/atc/engine"
	"github.com/concourse/atc/engine/fakes"
	"github.com/concourse/atc/event"
	"github.com/concourse/atc/exec"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildDelegate", func() {
	var (
		fakeDB  *fakes.FakeEngineDB
		factory BuildDelegateFactory

		buildID int

		delegate BuildDelegate

		logger *lagertest.TestLogger

		location event.OriginLocation
	)

	BeforeEach(func() {
		fakeDB = new(fakes.FakeEngineDB)
		factory = NewBuildDelegateFactory(fakeDB)

		buildID = 42
		delegate = factory.Delegate(buildID)

		logger = lagertest.NewTestLogger("test")

		location = event.OriginLocation{
			ParentID:      0,
			ID:            3,
			ParallelGroup: 1,
			Hook:          "some-hook",
		}
	})

	Describe("InputDelegate", func() {
		var (
			getPlan atc.GetPlan

			inputDelegate exec.GetDelegate
		)

		BeforeEach(func() {
			getPlan = atc.GetPlan{
				Name:     "some-input",
				Resource: "some-input-resource",
				Pipeline: "some-pipeline",
				Type:     "some-type",
				Version:  atc.Version{"some": "version"},
				Source:   atc.Source{"some": "source"},
				Params:   atc.Params{"some": "params"},
			}

			inputDelegate = delegate.InputDelegate(logger, getPlan, location)
		})

		Describe("Completed", func() {
			var versionInfo *exec.VersionInfo

			BeforeEach(func() {
				versionInfo = &exec.VersionInfo{
					Version:  atc.Version{"result": "version"},
					Metadata: []atc.MetadataField{{"result", "metadata"}},
				}
			})

			Context("when exit status is not 0", func() {
				JustBeforeEach(func() {
					inputDelegate.Completed(exec.ExitStatus(12), nil)
				})

				It("does not save the build's input", func() {
					Expect(fakeDB.SaveBuildInputCallCount()).To(Equal(0))
				})

				It("saves a finish-get event", func() {
					Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

					buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
					Expect(buildID).To(Equal(42))
					Expect(savedEvent).To(Equal(event.FinishGet{
						Origin: event.Origin{
							Type:     event.OriginTypeGet,
							Name:     "some-input",
							Location: location,
						},
						Plan: event.GetPlan{
							Name:     "some-input",
							Resource: "some-input-resource",
							Type:     "some-type",
							Version:  atc.Version{"some": "version"},
						},
						ExitStatus: 12,
					}))

				})
			})

			Context("when the version is null", func() {
				JustBeforeEach(func() {
					inputDelegate.Completed(exec.ExitStatus(12), nil)
				})

				It("does not save the build's input", func() {
					Expect(fakeDB.SaveBuildInputCallCount()).To(Equal(0))
				})

				It("saves a finish-get event", func() {
					Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

					buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
					Expect(buildID).To(Equal(42))
					Expect(savedEvent).To(Equal(event.FinishGet{
						Origin: event.Origin{
							Type:     event.OriginTypeGet,
							Name:     "some-input",
							Location: location,
						},
						Plan: event.GetPlan{
							Name:     "some-input",
							Resource: "some-input-resource",
							Type:     "some-type",
							Version:  atc.Version{"some": "version"},
						},
						ExitStatus:      12,
						FetchedVersion:  nil,
						FetchedMetadata: nil,
					}))
				})
			})

			Context("when the pipeline name is empty because of a one-off build", func() {
				BeforeEach(func() {
					getPlan.Pipeline = ""

					inputDelegate = delegate.InputDelegate(logger, getPlan, location)
				})

				JustBeforeEach(func() {
					inputDelegate.Completed(exec.ExitStatus(0), versionInfo)
				})

				It("does not save the build's input", func() {
					Expect(fakeDB.SaveBuildInputCallCount()).To(Equal(0))
				})

				It("saves a finish-get event", func() {
					Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

					buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
					Expect(buildID).To(Equal(42))
					Expect(savedEvent).To(Equal(event.FinishGet{
						Origin: event.Origin{
							Type:     event.OriginTypeGet,
							Name:     "some-input",
							Location: location,
						},
						Plan: event.GetPlan{
							Name:     "some-input",
							Resource: "some-input-resource",
							Type:     "some-type",
							Version:  atc.Version{"some": "version"},
						},
						ExitStatus:      0,
						FetchedVersion:  nil,
						FetchedMetadata: nil,
					}))
				})
			})

			Describe("Finish", func() {
				var (
					finishErr error
					aborted   bool
					succeeded exec.Success
				)

				Context("without error", func() {
					BeforeEach(func() {
						finishErr = nil
					})

					Context("when it was told it failed", func() {
						BeforeEach(func() {
							succeeded = false
							aborted = false
						})

						It("finishes with status 'failed'", func() {
							delegate.Finish(logger, finishErr, succeeded, aborted)

							Expect(fakeDB.FinishBuildCallCount()).To(Equal(1))

							buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
							Expect(buildID).To(Equal(42))
							Expect(savedStatus).To(Equal(db.StatusFailed))
						})
					})

					Context("when it was told it succeeded", func() {
						BeforeEach(func() {
							succeeded = true
						})

						It("finishes with status 'succeeded'", func() {
							delegate.Finish(logger, finishErr, succeeded, aborted)

							Expect(fakeDB.FinishBuildCallCount()).To(Equal(1))

							buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
							Expect(buildID).To(Equal(42))
							Expect(savedStatus).To(Equal(db.StatusSucceeded))
						})
					})
				})

				Context("when exit status is 0", func() {
					BeforeEach(func() {
						fakeDB.SaveBuildInputReturns(db.SavedVersionedResource{
							ID: 42,
							VersionedResource: db.VersionedResource{
								PipelineName: "some-pipeline",
								Resource:     "some-input-resource",
								Type:         "some-type",
								Version:      db.Version{"result": "version"},
								Metadata:     []db.MetadataField{{"saved", "metadata"}},
							},
						}, nil)
					})

					JustBeforeEach(func() {
						inputDelegate.Completed(exec.ExitStatus(0), versionInfo)
					})

					It("saves the build's input", func() {
						Expect(fakeDB.SaveBuildInputCallCount()).To(Equal(1))

						buildID, savedInput := fakeDB.SaveBuildInputArgsForCall(0)
						Expect(buildID).To(Equal(42))
						Expect(savedInput).To(Equal(db.BuildInput{
							Name: "some-input",
							VersionedResource: db.VersionedResource{
								PipelineName: "some-pipeline",
								Resource:     "some-input-resource",
								Type:         "some-type",
								Version:      db.Version{"result": "version"},
								Metadata:     []db.MetadataField{{"result", "metadata"}},
							},
						}))
					})

					It("saves a finish-get event", func() {
						Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

						buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
						Expect(buildID).To(Equal(42))
						Expect(savedEvent).To(Equal(event.FinishGet{
							Origin: event.Origin{
								Type:     event.OriginTypeGet,
								Name:     "some-input",
								Location: location,
							},
							Plan: event.GetPlan{
								Name:     "some-input",
								Resource: "some-input-resource",
								Type:     "some-type",
								Version:  atc.Version{"some": "version"},
							},
							FetchedVersion:  versionInfo.Version,
							FetchedMetadata: []atc.MetadataField{{"saved", "metadata"}},
						}))
					})

					Context("when the resource only occurs as an input", func() {
						Describe("Finish", func() {
							var (
								finishErr error
								aborted   bool
								succeeded exec.Success
							)

							Context("with success", func() {
								BeforeEach(func() {
									finishErr = nil
									succeeded = true
									aborted = false
								})

								It("saves the input as an implicit output", func() {
									delegate.Finish(logger, finishErr, succeeded, aborted)

									Expect(fakeDB.SaveBuildOutputCallCount()).To(Equal(1))

									buildID, savedOutput, explicit := fakeDB.SaveBuildOutputArgsForCall(0)
									Expect(buildID).To(Equal(42))
									Expect(savedOutput).To(Equal(db.VersionedResource{
										PipelineName: "some-pipeline",
										Resource:     "some-input-resource",
										Type:         "some-type",
										Version:      db.Version{"result": "version"},
										Metadata:     []db.MetadataField{{"result", "metadata"}},
									}))

									Expect(explicit).To(BeFalse())
								})
							})

							Context("with failure", func() {
								disaster := errors.New("nope")

								BeforeEach(func() {
									finishErr = disaster
									succeeded = false
								})

								It("does not save the input as an implicit output", func() {
									delegate.Finish(logger, finishErr, succeeded, aborted)

									Expect(fakeDB.SaveBuildOutputCallCount()).To(BeZero())
								})
							})
						})
					})

					Context("when the same resource occurs as an explicit output", func() {
						var (
							putPlan atc.PutPlan

							outputDelegate exec.PutDelegate
						)

						BeforeEach(func() {
							putPlan = atc.PutPlan{
								Pipeline: "some-pipeline",
								Resource: "some-input-resource",
								Type:     "some-type",
								Source:   atc.Source{"some": "source"},
								Params:   atc.Params{"some": "output-params"},
							}

							outputDelegate = delegate.OutputDelegate(logger, putPlan, location)
						})

						JustBeforeEach(func() {
							outputDelegate.Completed(exec.ExitStatus(0), &exec.VersionInfo{
								Version:  atc.Version{"explicit": "version"},
								Metadata: []atc.MetadataField{{"explicit", "metadata"}},
							})
						})

						Describe("Finish", func() {
							var (
								finishErr error
								succeeded exec.Success
							)

							BeforeEach(func() {
								finishErr = nil
								succeeded = true
							})

							It("only saves the explicit output", func() {
								delegate.Finish(logger, finishErr, succeeded, aborted)

								Expect(fakeDB.SaveBuildOutputCallCount()).To(Equal(1))

								buildID, savedOutput, explicit := fakeDB.SaveBuildOutputArgsForCall(0)
								Expect(buildID).To(Equal(42))
								Expect(savedOutput).To(Equal(db.VersionedResource{
									PipelineName: "some-pipeline",
									Resource:     "some-input-resource",
									Type:         "some-type",
									Version:      db.Version{"explicit": "version"},
									Metadata:     []db.MetadataField{{"explicit", "metadata"}},
								}))

								Expect(explicit).To(BeTrue())
							})

							Context("when the pipeline name is empty because of a one-off build", func() {
								BeforeEach(func() {
									putPlan.Pipeline = ""

									outputDelegate = delegate.OutputDelegate(logger, putPlan, location)
								})

								It("does not save it as an output", func() {
									delegate.Finish(logger, finishErr, succeeded, aborted)

									Expect(fakeDB.SaveBuildOutputCallCount()).To(BeZero())
								})
							})
						})
					})
				})
			})
		})

		Describe("Failed", func() {
			JustBeforeEach(func() {
				inputDelegate.Failed(errors.New("nope"))
			})

			It("does not save the build's input", func() {
				Expect(fakeDB.SaveBuildInputCallCount()).To(BeZero())
			})

			It("saves an error event", func() {
				Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Expect(buildID).To(Equal(42))
				Expect(savedEvent).To(Equal(event.Error{
					Origin: event.Origin{
						Type:     event.OriginTypeGet,
						Name:     "some-input",
						Location: location,
					},
					Message: "nope",
				}))

			})
		})

		Describe("Stdout", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = inputDelegate.Stdout()
			})

			It("saves log events with the input's origin", func() {
				_, err := writer.Write([]byte("some stdout"))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

				savedBuildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Expect(savedBuildID).To(Equal(buildID))
				Expect(savedEvent).To(Equal(event.Log{
					Origin: event.Origin{
						Type:     event.OriginTypeGet,
						Name:     "some-input",
						Source:   event.OriginSourceStdout,
						Location: location,
					},
					Payload: "some stdout",
				}))

			})
		})

		Describe("Stderr", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = inputDelegate.Stderr()
			})

			It("saves log events with the input's origin", func() {
				_, err := writer.Write([]byte("some stderr"))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

				savedBuildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Expect(savedBuildID).To(Equal(buildID))
				Expect(savedEvent).To(Equal(event.Log{
					Origin: event.Origin{
						Type:     event.OriginTypeGet,
						Name:     "some-input",
						Source:   event.OriginSourceStderr,
						Location: location,
					},
					Payload: "some stderr",
				}))

			})
		})
	})

	Describe("ExecutionDelegate", func() {
		var (
			taskPlan          atc.TaskPlan
			executionDelegate exec.TaskDelegate
		)

		BeforeEach(func() {
			taskPlan = atc.TaskPlan{
				Name:       "some-task",
				Privileged: true,
				ConfigPath: "/etc/concourse/config.yml",
			}

			executionDelegate = delegate.ExecutionDelegate(logger, taskPlan, location)
		})

		Describe("Initializing", func() {
			var taskConfig atc.TaskConfig

			BeforeEach(func() {
				taskConfig = atc.TaskConfig{
					Run: atc.TaskRunConfig{
						Path: "ls",
					},
				}
			})

			JustBeforeEach(func() {
				executionDelegate.Initializing(taskConfig)
			})

			It("saves an initialize event", func() {
				Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Expect(buildID).To(Equal(42))
				Expect(savedEvent).To(Equal(event.InitializeTask{
					TaskConfig: event.TaskConfig{
						Run: event.TaskRunConfig{
							Path: "ls",
						},
					},
					Origin: event.Origin{
						Type:     event.OriginTypeTask,
						Name:     "some-task",
						Location: location,
					},
				}))

			})
		})

		Describe("Started", func() {
			JustBeforeEach(func() {
				executionDelegate.Started()
			})

			It("saves a start event", func() {
				Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Expect(buildID).To(Equal(42))
				Expect(savedEvent).To(BeAssignableToTypeOf(event.StartTask{}))
				Expect(savedEvent.(event.StartTask).Time).To(BeNumerically("~", time.Now().Unix(), 1))
				Expect(savedEvent.(event.StartTask).Origin).To(Equal(event.Origin{
					Type:     event.OriginTypeTask,
					Name:     "some-task",
					Location: location,
				}))

			})
		})

		Describe("Finished", func() {
			var exitStatus exec.ExitStatus

			JustBeforeEach(func() {
				executionDelegate.Finished(exitStatus)
			})

			Context("with a successful result", func() {
				BeforeEach(func() {
					exitStatus = 0
				})

				It("saves a finish event", func() {
					Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

					buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
					Expect(buildID).To(Equal(42))
					Expect(savedEvent).To(BeAssignableToTypeOf(event.FinishTask{}))
					Expect(savedEvent.(event.FinishTask).ExitStatus).To(Equal(0))
					Expect(savedEvent.(event.FinishTask).Time).To(BeNumerically("<=", time.Now().Unix(), 1))
					Expect(savedEvent.(event.FinishTask).Origin).To(Equal(event.Origin{
						Type:     event.OriginTypeTask,
						Name:     "some-task",
						Location: location,
					}))

				})

				Describe("Finish", func() {
					var (
						finishErr error
						aborted   bool
						succeeded exec.Success
					)

					Context("with success", func() {
						BeforeEach(func() {
							finishErr = nil
							succeeded = true
							aborted = false
						})

						It("finishes with status 'succeeded'", func() {
							delegate.Finish(logger, finishErr, succeeded, aborted)

							Expect(fakeDB.FinishBuildCallCount()).To(Equal(1))

							buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
							Expect(buildID).To(Equal(42))
							Expect(savedStatus).To(Equal(db.StatusSucceeded))
						})
					})

					Context("with failure", func() {
						disaster := errors.New("nope")

						BeforeEach(func() {
							finishErr = disaster
							succeeded = false
						})

						It("finishes with status 'errored'", func() {
							delegate.Finish(logger, finishErr, succeeded, aborted)

							Expect(fakeDB.FinishBuildCallCount()).To(Equal(1))

							buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
							Expect(buildID).To(Equal(42))
							Expect(savedStatus).To(Equal(db.StatusErrored))
						})
					})
				})
			})

			Context("with a failed result", func() {
				BeforeEach(func() {
					exitStatus = 1
				})

				It("saves a finish event", func() {
					Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

					buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
					Expect(buildID).To(Equal(42))
					Expect(savedEvent).To(BeAssignableToTypeOf(event.FinishTask{}))
					Expect(savedEvent.(event.FinishTask).ExitStatus).To(Equal(1))
					Expect(savedEvent.(event.FinishTask).Time).To(BeNumerically("<=", time.Now().Unix(), 1))
					Expect(savedEvent.(event.FinishTask).Origin).To(Equal(event.Origin{
						Type:     event.OriginTypeTask,
						Name:     "some-task",
						Location: location,
					}))

				})
			})
		})

		Describe("Failed", func() {
			JustBeforeEach(func() {
				executionDelegate.Failed(errors.New("nope"))
			})

			It("does not save the build's input", func() {
				Expect(fakeDB.SaveBuildInputCallCount()).To(BeZero())
			})

			It("saves an error event", func() {
				Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Expect(buildID).To(Equal(42))
				Expect(savedEvent).To(Equal(event.Error{
					Message: "nope",
					Origin: event.Origin{
						Type:     event.OriginTypeTask,
						Name:     "some-task",
						Location: location,
					},
				}))

			})
		})

		Describe("Stdout", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = executionDelegate.Stdout()
			})

			It("saves log events with the correct origin", func() {
				_, err := writer.Write([]byte("some stdout"))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

				savedBuildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Expect(savedBuildID).To(Equal(buildID))
				Expect(savedEvent).To(Equal(event.Log{
					Origin: event.Origin{
						Type:     event.OriginTypeTask,
						Name:     "some-task",
						Source:   event.OriginSourceStdout,
						Location: location,
					},
					Payload: "some stdout",
				}))

			})
		})

		Describe("Stderr", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = executionDelegate.Stderr()
			})

			It("saves log events with the correct origin", func() {
				_, err := writer.Write([]byte("some stderr"))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

				savedBuildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Expect(savedBuildID).To(Equal(buildID))
				Expect(savedEvent).To(Equal(event.Log{
					Origin: event.Origin{
						Type:     event.OriginTypeTask,
						Name:     "some-task",
						Source:   event.OriginSourceStderr,
						Location: location,
					},
					Payload: "some stderr",
				}))

			})
		})
	})

	Describe("OutputDelegate", func() {
		var (
			putPlan atc.PutPlan

			outputDelegate exec.PutDelegate
		)

		BeforeEach(func() {
			putPlan = atc.PutPlan{
				Name:     "some-output-name",
				Resource: "some-output-resource",
				Pipeline: "some-other-pipeline",
				Type:     "some-type",
				Source:   atc.Source{"some": "source"},
				Params:   atc.Params{"some": "params"},
			}

			outputDelegate = delegate.OutputDelegate(logger, putPlan, location)
		})

		Describe("Completed", func() {
			var versionInfo *exec.VersionInfo

			BeforeEach(func() {
				versionInfo = &exec.VersionInfo{
					Version:  atc.Version{"result": "version"},
					Metadata: []atc.MetadataField{{"result", "metadata"}},
				}
			})

			Context("when the version info is nil", func() {
				JustBeforeEach(func() {
					outputDelegate.Completed(exec.ExitStatus(0), nil)
				})

				It("does not save the build's output", func() {
					Expect(fakeDB.SaveBuildOutputCallCount()).To(Equal(0))
				})

				It("saves an output event", func() {
					Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

					buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
					Expect(buildID).To(Equal(42))
					Expect(savedEvent).To(Equal(event.FinishPut{
						Origin: event.Origin{
							Type:     event.OriginTypePut,
							Name:     "some-output-name",
							Location: location,
						},
						Plan: event.PutPlan{
							Name:     "some-output-name",
							Resource: "some-output-resource",
							Type:     "some-type",
						},
						ExitStatus:      0,
						CreatedVersion:  nil,
						CreatedMetadata: nil,
					}))

				})
			})

			Context("when exit status is 0", func() {
				JustBeforeEach(func() {
					outputDelegate.Completed(exec.ExitStatus(0), versionInfo)
				})

				It("saves the build's output", func() {
					Expect(fakeDB.SaveBuildOutputCallCount()).To(Equal(1))

					buildID, savedOutput, explicit := fakeDB.SaveBuildOutputArgsForCall(0)
					Expect(buildID).To(Equal(42))
					Expect(savedOutput).To(Equal(db.VersionedResource{
						PipelineName: "some-other-pipeline",
						Resource:     "some-output-resource",
						Type:         "some-type",
						Version:      db.Version{"result": "version"},
						Metadata:     []db.MetadataField{{"result", "metadata"}},
					}))

					Expect(explicit).To(BeTrue())
				})

				It("saves an output event", func() {
					Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

					buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
					Expect(buildID).To(Equal(42))
					Expect(savedEvent).To(Equal(event.FinishPut{
						Origin: event.Origin{
							Type:     event.OriginTypePut,
							Name:     "some-output-name",
							Location: location,
						},
						Plan: event.PutPlan{
							Name:     "some-output-name",
							Resource: "some-output-resource",
							Type:     "some-type",
						},
						CreatedVersion:  versionInfo.Version,
						CreatedMetadata: versionInfo.Metadata,
						ExitStatus:      0,
					}))

				})
			})

			Context("when exit status is not 0", func() {
				JustBeforeEach(func() {
					outputDelegate.Completed(exec.ExitStatus(72), versionInfo)
				})

				It("saves the build's output", func() {
					Expect(fakeDB.SaveBuildOutputCallCount()).To(Equal(1))

					buildID, savedOutput, explicit := fakeDB.SaveBuildOutputArgsForCall(0)
					Expect(buildID).To(Equal(42))
					Expect(savedOutput).To(Equal(db.VersionedResource{
						PipelineName: "some-other-pipeline",
						Resource:     "some-output-resource",
						Type:         "some-type",
						Version:      db.Version{"result": "version"},
						Metadata:     []db.MetadataField{{"result", "metadata"}},
					}))

					Expect(explicit).To(BeTrue())
				})

				It("saves an output event", func() {
					Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

					buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
					Expect(buildID).To(Equal(42))
					Expect(savedEvent).To(Equal(event.FinishPut{
						Origin: event.Origin{
							Type:     event.OriginTypePut,
							Name:     "some-output-name",
							Location: location,
						},
						Plan: event.PutPlan{
							Name:     "some-output-name",
							Resource: "some-output-resource",
							Type:     "some-type",
						},
						CreatedVersion:  versionInfo.Version,
						CreatedMetadata: versionInfo.Metadata,
						ExitStatus:      72,
					}))

				})

			})

			Describe("Finish", func() {
				var (
					finishErr error
					aborted   bool
					succeeded exec.Success
				)

				Context("without error", func() {
					BeforeEach(func() {
						finishErr = nil
					})

					Context("when it was told it succeeded", func() {
						BeforeEach(func() {
							succeeded = true
							aborted = false
						})

						It("finishes with status 'failed'", func() {
							delegate.Finish(logger, finishErr, succeeded, aborted)

							Expect(fakeDB.FinishBuildCallCount()).To(Equal(1))

							buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
							Expect(buildID).To(Equal(42))
							Expect(savedStatus).To(Equal(db.StatusSucceeded))
						})
					})

					Context("when it was told it failed", func() {
						BeforeEach(func() {
							succeeded = false
						})

						It("finishes with status 'failed'", func() {
							delegate.Finish(logger, finishErr, succeeded, aborted)

							Expect(fakeDB.FinishBuildCallCount()).To(Equal(1))

							buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
							Expect(buildID).To(Equal(42))
							Expect(savedStatus).To(Equal(db.StatusFailed))
						})
					})
				})
			})
		})

		Describe("Failed", func() {
			JustBeforeEach(func() {
				outputDelegate.Failed(errors.New("nope"))
			})

			It("does not save the build's input", func() {
				Expect(fakeDB.SaveBuildInputCallCount()).To(BeZero())
			})

			It("saves an error event", func() {
				Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

				buildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Expect(buildID).To(Equal(42))
				Expect(savedEvent).To(Equal(event.Error{
					Origin: event.Origin{
						Type:     event.OriginTypePut,
						Name:     "some-output-name",
						Location: location,
					},
					Message: "nope",
				}))

			})
		})

		Describe("Stdout", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = outputDelegate.Stdout()
			})

			It("saves log events with the output's origin", func() {
				_, err := writer.Write([]byte("some stdout"))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

				savedBuildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Expect(savedBuildID).To(Equal(buildID))
				Expect(savedEvent).To(Equal(event.Log{
					Origin: event.Origin{
						Type:     event.OriginTypePut,
						Name:     "some-output-name",
						Source:   event.OriginSourceStdout,
						Location: location,
					},
					Payload: "some stdout",
				}))

			})
		})

		Describe("Stderr", func() {
			var writer io.Writer

			BeforeEach(func() {
				writer = outputDelegate.Stderr()
			})

			It("saves log events with the output's origin", func() {
				_, err := writer.Write([]byte("some stderr"))
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeDB.SaveBuildEventCallCount()).To(Equal(1))

				savedBuildID, savedEvent := fakeDB.SaveBuildEventArgsForCall(0)
				Expect(savedBuildID).To(Equal(buildID))
				Expect(savedEvent).To(Equal(event.Log{
					Origin: event.Origin{
						Type:     event.OriginTypePut,
						Name:     "some-output-name",
						Source:   event.OriginSourceStderr,
						Location: location,
					},
					Payload: "some stderr",
				}))

			})
		})
	})

	Describe("Aborted", func() {
		var aborted bool

		JustBeforeEach(func() {
			aborted = true
		})

		Describe("Finish", func() {
			var (
				finishErr error
				succeeded exec.Success
				// aborted   bool
			)

			Context("with success", func() {
				BeforeEach(func() {
					finishErr = nil
					succeeded = true

				})

				It("finishes with status 'aborted'", func() {
					delegate.Finish(logger, finishErr, succeeded, aborted)

					Expect(fakeDB.FinishBuildCallCount()).To(Equal(1))

					buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
					Expect(buildID).To(Equal(42))
					Expect(savedStatus).To(Equal(db.StatusAborted))
				})
			})

			Context("with failure", func() {
				disaster := errors.New("nope")

				BeforeEach(func() {
					finishErr = disaster
					succeeded = false
				})

				It("finishes with status 'aborted'", func() {
					delegate.Finish(logger, finishErr, succeeded, aborted)

					Expect(fakeDB.FinishBuildCallCount()).To(Equal(1))

					buildID, savedStatus := fakeDB.FinishBuildArgsForCall(0)
					Expect(buildID).To(Equal(42))
					Expect(savedStatus).To(Equal(db.StatusAborted))
				})
			})
		})
	})
})
