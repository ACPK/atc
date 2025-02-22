package atc

import "fmt"

const ConfigVersionHeader = "X-Concourse-Config-Version"
const DefaultPipelineName = "main"

type Source map[string]interface{}
type Params map[string]interface{}
type Version map[string]interface{}
type Tags []string

type Config struct {
	Groups    GroupConfigs    `yaml:"groups" json:"groups" mapstructure:"groups"`
	Resources ResourceConfigs `yaml:"resources" json:"resources" mapstructure:"resources"`
	Jobs      JobConfigs      `yaml:"jobs" json:"jobs" mapstructure:"jobs"`
}

type GroupConfig struct {
	Name      string   `yaml:"name" json:"name" mapstructure:"name"`
	Jobs      []string `yaml:"jobs,omitempty" json:"jobs,omitempty" mapstructure:"jobs"`
	Resources []string `yaml:"resources,omitempty" json:"resources,omitempty" mapstructure:"resources"`
}

type GroupConfigs []GroupConfig

func (groups GroupConfigs) Lookup(name string) (GroupConfig, bool) {
	for _, group := range groups {
		if group.Name == name {
			return group, true
		}
	}

	return GroupConfig{}, false
}

type ResourceConfig struct {
	Name string `yaml:"name" json:"name" mapstructure:"name"`

	Type       string `yaml:"type" json:"type" mapstructure:"type"`
	Source     Source `yaml:"source" json:"source" mapstructure:"source"`
	CheckEvery string `yaml:"check_every,omitempty" json:"check_every" mapstructure:"check_every"`
}

type JobConfig struct {
	Name   string `yaml:"name" json:"name" mapstructure:"name"`
	Public bool   `yaml:"public,omitempty" json:"public,omitempty" mapstructure:"public"`

	Serial         bool     `yaml:"serial,omitempty" json:"serial,omitempty" mapstructure:"serial"`
	SerialGroups   []string `yaml:"serial_groups,omitempty" json:"serial_groups,omitempty" mapstructure:"serial_groups"`
	RawMaxInFlight int      `yaml:"max_in_flight,omitempty" json:"max_in_flight,omitempty" mapstructure:"max_in_flight"`

	Plan PlanSequence `yaml:"plan,omitempty" json:"plan,omitempty" mapstructure:"plan"`
}

func (config JobConfig) MaxInFlight() int {
	if config.Serial || len(config.SerialGroups) > 0 {
		return 1
	}

	if config.RawMaxInFlight != 0 {
		return config.RawMaxInFlight
	}

	return 0
}

func (config JobConfig) GetSerialGroups() []string {
	if len(config.SerialGroups) > 0 {
		return config.SerialGroups
	}

	if config.Serial || config.RawMaxInFlight > 0 {
		return []string{config.Name}
	}

	return []string{}
}

// A PlanSequence corresponds to a chain of Compose plan, with an implicit
// `on: [success]` after every Task plan.
type PlanSequence []PlanConfig

// A PlanConfig is a flattened set of configuration corresponding to
// a particular Plan, where Source and Version are populated lazily.
type PlanConfig struct {
	// makes the Plan conditional
	// conditions on which to perform a nested sequence

	// compose a nested sequence of plans
	// name of the nested 'do'
	RawName string `yaml:"name,omitempty" json:"name,omitempty" mapstructure:"name"`

	// a nested chain of steps to run
	Do *PlanSequence `yaml:"do,omitempty" json:"do,omitempty" mapstructure:"do"`

	// corresponds to an Aggregate plan, keyed by the name of each sub-plan
	Aggregate *PlanSequence `yaml:"aggregate,omitempty" json:"aggregate,omitempty" mapstructure:"aggregate"`

	// corresponds to Get and Put resource plans, respectively
	// name of 'input', e.g. bosh-stemcell
	Get string `yaml:"get,omitempty" json:"get,omitempty" mapstructure:"get"`
	// jobs that this resource must have made it through
	Passed []string `yaml:"passed,omitempty" json:"passed,omitempty" mapstructure:"passed"`
	// whether to trigger based on this resource changing
	Trigger bool `yaml:"trigger,omitempty" json:"trigger,omitempty" mapstructure:"trigger"`

	// name of 'output', e.g. rootfs-tarball
	Put string `yaml:"put,omitempty" json:"put,omitempty" mapstructure:"put"`

	// corresponding resource config, e.g. aws-stemcell
	Resource string `yaml:"resource,omitempty" json:"resource,omitempty" mapstructure:"resource"`

	// corresponds to a Task plan
	// name of 'task', e.g. unit, go1.3, go1.4
	Task string `yaml:"task,omitempty" json:"task,omitempty" mapstructure:"task"`
	// run task privileged
	Privileged bool `yaml:"privileged,omitempty" json:"privileged,omitempty" mapstructure:"privileged"`
	// task config path, e.g. foo/build.yml
	TaskConfigPath string `yaml:"file,omitempty" json:"file,omitempty" mapstructure:"file"`
	// inlined task config
	TaskConfig *TaskConfig `yaml:"config,omitempty" json:"config,omitempty" mapstructure:"config"`

	// used by Get and Put for specifying params to the resource
	Params Params `yaml:"params,omitempty" json:"params,omitempty" mapstructure:"params"`

	// used by Put to specify params for the subsequent Get
	GetParams Params `yaml:"get_params,omitempty" json:"get_params,omitempty" mapstructure:"get_params"`

	// used by any step to specify which workers are eligible to run the step
	Tags Tags `yaml:"tags,omitempty" json:"tags,omitempty" mapstructure:"tags"`

	// used by any step to run something when the step reports a failure
	Failure *PlanConfig `yaml:"on_failure,omitempty" json:"on_failure,omitempty" mapstructure:"on_failure"`

	// used on any step to always execute regardless of the step's completed state
	Ensure *PlanConfig `yaml:"ensure,omitempty" json:"ensure,omitempty" mapstructure:"ensure"`

	// used on any step to execute on successful completion of the step
	Success *PlanConfig `yaml:"on_success,omitempty" json:"on_success,omitempty" mapstructure:"on_success"`

	// used on any step to swallow failures and errors
	Try *PlanConfig `yaml:"try,omitempty" json:"try,omitempty" mapstructure:"try"`

	// used on any step to interrupt the step after a given duration
	Timeout string `yaml:"timeout,omitempty" json:"timeout,omitempty" mapstructure:"timeout"`

	// not present in yaml
	Location *Location `yaml:"-" json:"-"`

	// not present in yaml
	DependentGet string `yaml:"-" json:"-"`
}

func (config PlanConfig) Name() string {
	if config.RawName != "" {
		return config.RawName
	}

	if config.Get != "" {
		return config.Get
	}

	if config.Put != "" {
		return config.Put
	}

	if config.Task != "" {
		return config.Task
	}

	return ""
}

func (config PlanConfig) ResourceName() string {
	resourceName := config.Resource
	if resourceName != "" {
		return resourceName
	}

	resourceName = config.Get
	if resourceName != "" {
		return resourceName
	}

	resourceName = config.Put
	if resourceName != "" {
		return resourceName
	}

	panic("no resource name!")
}

type ResourceConfigs []ResourceConfig

func (resources ResourceConfigs) Lookup(name string) (ResourceConfig, bool) {
	for _, resource := range resources {
		if resource.Name == name {
			return resource, true
		}
	}

	return ResourceConfig{}, false
}

type JobConfigs []JobConfig

func (jobs JobConfigs) Lookup(name string) (JobConfig, bool) {
	for _, job := range jobs {
		if job.Name == name {
			return job, true
		}
	}

	return JobConfig{}, false
}

func (config Config) JobIsPublic(jobName string) (bool, error) {
	job, found := config.Jobs.Lookup(jobName)
	if !found {
		return false, fmt.Errorf("cannot find job with job name '%s'", jobName)
	}

	return job.Public, nil
}
