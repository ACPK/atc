package atc

type Worker struct {
	Addr string `json:"addr"`

	ActiveContainers int `json:"active_containers"`

	ResourceTypes []WorkerResourceType `json:"resource_types"`

	Platform string   `json:"platform"`
	Tags     []string `json:"tags"`
	Name     string   `json:"name"`
}

type WorkerResourceType struct {
	Type  string `json:"type"`
	Image string `json:"image"`
}
