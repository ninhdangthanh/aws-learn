package main

import "fmt"

// Step is one unit of work in a saga: something that can succeed, plus the
// action that undoes it afterwards.
//
// Compensate is not a rollback. By the time it runs, Do has already committed
// - money moved, stock left the shelf. Compensate issues a *new* action that
// cancels out the old one (refund, restock), which is why it can itself fail
// and why it must be safe to reason about on its own.
type Step struct {
	Name       string
	Do         func(orderID string) error
	Compensate func(orderID string) error
}

// Saga runs steps in order. If any step fails, every step that already
// succeeded is compensated in reverse order.
type Saga struct {
	steps []Step
	log   func(string)
}

func NewSaga(log func(string), steps ...Step) *Saga {
	if log == nil {
		log = func(string) {}
	}
	return &Saga{steps: steps, log: log}
}

// Result records what the saga did, so callers (and tests) can assert on the
// exact sequence rather than guessing from side effects.
type Result struct {
	Completed    []string // steps that succeeded, in execution order
	Compensated  []string // steps undone, in the order they were undone
	FailedStep   string   // step that broke the chain, empty if the saga succeeded
	FailureCause error
}

func (r Result) OK() bool { return r.FailedStep == "" }

func (s *Saga) Execute(orderID string) Result {
	res := Result{}

	for _, step := range s.steps {
		if err := step.Do(orderID); err != nil {
			s.log(fmt.Sprintf("  x %-16s failed: %v", step.Name, err))
			res.FailedStep = step.Name
			res.FailureCause = err
			s.compensate(orderID, &res)
			return res
		}
		s.log(fmt.Sprintf("  > %-16s ok", step.Name))
		res.Completed = append(res.Completed, step.Name)
	}

	return res
}

// compensate walks the completed steps backwards. Reverse order matters: a
// later step may depend on an earlier one, so undoing the earlier one first
// could leave the system in a state the later compensation cannot handle.
func (s *Saga) compensate(orderID string, res *Result) {
	for i := len(res.Completed) - 1; i >= 0; i-- {
		name := res.Completed[i]
		step := s.stepByName(name)

		if step.Compensate == nil {
			s.log(fmt.Sprintf("  - %-16s nothing to compensate", name))
			continue
		}

		if err := step.Compensate(orderID); err != nil {
			// A failed compensation is the nightmare case: the system is now
			// genuinely inconsistent and no amount of retrying in-process
			// fixes it. Real systems park this on a dead-letter queue and page
			// a human. Here we just make it loud.
			s.log(fmt.Sprintf("  ! %-16s COMPENSATION FAILED: %v (manual intervention required)", name, err))
			continue
		}
		s.log(fmt.Sprintf("  < %-16s compensated", name))
		res.Compensated = append(res.Compensated, name)
	}
}

func (s *Saga) stepByName(name string) Step {
	for _, st := range s.steps {
		if st.Name == name {
			return st
		}
	}
	return Step{}
}
