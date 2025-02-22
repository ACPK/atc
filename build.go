package atc

type BuildStatus string

const (
	StatusStarted   BuildStatus = "started"
	StatusPending   BuildStatus = "pending"
	StatusSucceeded BuildStatus = "succeeded"
	StatusFailed    BuildStatus = "failed"
	StatusErrored   BuildStatus = "errored"
	StatusAborted   BuildStatus = "aborted"
)

type Build struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	JobName      string `json:"job_name,omitempty"`
	URL          string `json:"url"`
	APIURL       string `json:"api_url"`
	PipelineName string `json:"pipeline_name,omitempty"`
	StartTime    int64  `json:"start_time,omitempty"`
	EndTime      int64  `json:"end_time,omitempty"`
}

func (b Build) IsRunning() bool {
	switch BuildStatus(b.Status) {
	case StatusPending, StatusStarted:
		return true
	default:
		return false
	}
}

func (b Build) Abortable() bool {
	return b.IsRunning()
}

func (b Build) OneOff() bool {
	return b.JobName == ""
}
