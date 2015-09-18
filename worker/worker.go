package worker

type Worker interface {
	Client

	ActiveContainers() int
	Satisfies(ContainerSpec) bool

	Description() string
}
