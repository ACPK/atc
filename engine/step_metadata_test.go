package engine_test

import (
	. "github.com/concourse/atc/engine"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("StepMetadata", func() {
	Describe("Env", func() {
		It("returns the specified values", func() {
			Expect(StepMetadata{
				BuildID:      1,
				PipelineName: "some-pipeline-name",
				JobName:      "some-job-name",
				BuildName:    "42",
			}.Env()).To(Equal([]string{
				"BUILD_ID=1",
				"BUILD_PIPELINE_NAME=some-pipeline-name",
				"BUILD_JOB_NAME=some-job-name",
				"BUILD_NAME=42",
			}))
		})

		It("does not include fields that are not set", func() {
			Expect(StepMetadata{
				BuildID: 1,
			}.Env()).To(Equal([]string{
				"BUILD_ID=1",
			}))
		})
	})
})
