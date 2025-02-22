package engine

import "fmt"

type StepMetadata struct {
	BuildID int

	PipelineName string
	JobName      string
	BuildName    string
}

func (metadata StepMetadata) Env() []string {
	env := []string{fmt.Sprintf("BUILD_ID=%d", metadata.BuildID)}

	if metadata.PipelineName != "" {
		env = append(env, "BUILD_PIPELINE_NAME="+metadata.PipelineName)
	}

	if metadata.JobName != "" {
		env = append(env, "BUILD_JOB_NAME="+metadata.JobName)
	}

	if metadata.BuildName != "" {
		env = append(env, "BUILD_NAME="+metadata.BuildName)
	}

	return env
}
