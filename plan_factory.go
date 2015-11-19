package atc

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
)

type PlanFactory struct {
	currentNum *int64
}

func NewPlanFactory(startingNum int64) PlanFactory {
	return PlanFactory{
		currentNum: &startingNum,
	}
}

type Step interface {
	Public() *json.RawMessage
}

func (factory PlanFactory) NewPlan(step Step) Plan {
	num := atomic.AddInt64(factory.currentNum, 1)

	var plan Plan
	switch t := step.(type) {
	case AggregatePlan:
		plan.Aggregate = &t
	case GetPlan:
		plan.Get = &t
	case PutPlan:
		plan.Put = &t
	case TaskPlan:
		plan.Task = &t
	case EnsurePlan:
		plan.Ensure = &t
	case OnSuccessPlan:
		plan.OnSuccess = &t
	case OnFailurePlan:
		plan.OnFailure = &t
	case TryPlan:
		plan.Try = &t
	case DependentGetPlan:
		plan.DependentGet = &t
	case TimeoutPlan:
		plan.Timeout = &t
	default:
		panic("huh.")
	}

	plan.ID = PlanID(fmt.Sprintf("%x", num))

	return plan
}
