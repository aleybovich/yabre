package yabre

import "time"

// TraceEntry records a single condition evaluation step during a RunRulesWithTrace call.
type TraceEntry struct {
	// ConditionName is the name of the evaluated condition.
	ConditionName string
	// Description is the condition's human-readable description.
	Description string
	// Result is the boolean outcome of the condition's check function.
	Result bool
	// HasAction indicates whether the matched branch has an action defined.
	HasAction bool
	// NextCondition is the name of the condition that was chained after this one,
	// taken directly from the Decision's Next field. Empty when there is no next.
	NextCondition string
	// Terminated reports whether execution stopped at this condition (either via
	// the Decision's Terminate flag or because no Decision branch was defined).
	Terminated bool
	// Duration is the wall-clock time spent evaluating the check function plus
	// the direct action for this condition. Time spent in downstream conditions
	// is NOT included; those appear as their own entries.
	Duration time.Duration
	// Error holds the first error encountered while evaluating this condition
	// (check or action). Nil on success.
	Error error
}

// traceCollector is a per-call accumulator for trace entries. It is created
// inside RunRulesWithTrace and passed through runCondition/runAction by pointer.
// It is never stored on RulesRunner, ensuring goroutine-safety.
type traceCollector struct {
	entries []TraceEntry
}
