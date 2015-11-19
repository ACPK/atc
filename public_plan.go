package atc

import "encoding/json"

func (plan Plan) Public() *json.RawMessage {
	var public struct {
		ID PlanID `json:"id"`

		Aggregate    *json.RawMessage `json:"aggregate,omitempty"`
		Get          *json.RawMessage `json:"get,omitempty"`
		Put          *json.RawMessage `json:"put,omitempty"`
		Task         *json.RawMessage `json:"task,omitempty"`
		Ensure       *json.RawMessage `json:"ensure,omitempty"`
		OnSuccess    *json.RawMessage `json:"on_success,omitempty"`
		OnFailure    *json.RawMessage `json:"on_failure,omitempty"`
		Try          *json.RawMessage `json:"try,omitempty"`
		DependentGet *json.RawMessage `json:"dependent_get,omitempty"`
		Timeout      *json.RawMessage `json:"timeout,omitempty"`
	}

	public.ID = plan.ID

	if plan.Aggregate != nil {
		public.Aggregate = plan.Aggregate.Public()
	}

	if plan.Get != nil {
		public.Get = plan.Get.Public()
	}

	if plan.Put != nil {
		public.Put = plan.Put.Public()
	}

	if plan.Task != nil {
		public.Task = plan.Task.Public()
	}

	if plan.Ensure != nil {
		public.Ensure = plan.Ensure.Public()
	}

	if plan.OnSuccess != nil {
		public.OnSuccess = plan.OnSuccess.Public()
	}

	if plan.OnFailure != nil {
		public.OnFailure = plan.OnFailure.Public()
	}

	if plan.Try != nil {
		public.Try = plan.Try.Public()
	}

	if plan.DependentGet != nil {
		public.DependentGet = plan.DependentGet.Public()
	}

	if plan.Timeout != nil {
		public.Timeout = plan.Timeout.Public()
	}

	return enc(public)
}

func (plan AggregatePlan) Public() *json.RawMessage {
	public := make([]*json.RawMessage, len(plan))

	for i := 0; i < len(plan); i++ {
		public[i] = plan[i].Public()
	}

	return enc(public)
}

func (plan DependentGetPlan) Public() *json.RawMessage {
	return enc(struct {
		Type     string `json:"type"`
		Name     string `json:"name,omitempty"`
		Resource string `json:"resource"`
	}{
		Type:     plan.Type,
		Name:     plan.Name,
		Resource: plan.Resource,
	})
}

func (plan EnsurePlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
		Next *json.RawMessage `json:"ensure"`
	}{
		Step: plan.Step.Public(),
		Next: plan.Next.Public(),
	})
}

func (plan GetPlan) Public() *json.RawMessage {
	return enc(struct {
		Type     string  `json:"type"`
		Name     string  `json:"name,omitempty"`
		Resource string  `json:"resource"`
		Version  Version `json:"version,omitempty"`
	}{
		Type:     plan.Type,
		Name:     plan.Name,
		Resource: plan.Resource,
		Version:  plan.Version,
	})
}

func (plan OnFailurePlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
		Next *json.RawMessage `json:"on_failure"`
	}{
		Step: plan.Step.Public(),
		Next: plan.Next.Public(),
	})
}

func (plan OnSuccessPlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
		Next *json.RawMessage `json:"on_success"`
	}{
		Step: plan.Step.Public(),
		Next: plan.Next.Public(),
	})
}

func (plan PutPlan) Public() *json.RawMessage {
	return enc(struct {
		Type     string `json:"type"`
		Name     string `json:"name,omitempty"`
		Resource string `json:"resource"`
	}{
		Type:     plan.Type,
		Name:     plan.Name,
		Resource: plan.Resource,
	})
}

func (plan TaskPlan) Public() *json.RawMessage {
	return enc(struct {
		Name       string `json:"name"`
		Privileged bool   `json:"privileged"`
	}{
		Name:       plan.Name,
		Privileged: plan.Privileged,
	})
}

func (plan TimeoutPlan) Public() *json.RawMessage {
	return enc(struct {
		Step     *json.RawMessage `json:"step"`
		Duration string           `json:"duration"`
	}{
		Step:     plan.Step.Public(),
		Duration: plan.Duration,
	})
}

func (plan TryPlan) Public() *json.RawMessage {
	return enc(struct {
		Step *json.RawMessage `json:"step"`
	}{
		Step: plan.Step.Public(),
	})
}

func enc(public interface{}) *json.RawMessage {
	enc, _ := json.Marshal(public)
	return (*json.RawMessage)(&enc)
}
