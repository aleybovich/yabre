# Problems to solve

## 1. Add context cancellation to rule execution

Yabre runs user-provided JavaScript, so callers need a reliable way to stop rules that take too long or loop forever. Add these context-aware execution methods:

```go
func (rr *RulesRunner[Context]) RunRulesCtx(
    ctx context.Context,
    rulesContext *Context,
    startCondition *Condition,
) (*Context, error)

func (rr *RulesRunner[Context]) RunRulesWithTraceCtx(
    ctx context.Context,
    rulesContext *Context,
    startCondition *Condition,
) (*Context, []TraceEntry, error)
```

Keep the existing execution methods compatible by making them behave as though they use `context.Background()`.

The implementation must:

- Return immediately when given an already-cancelled context, without executing scripts, conditions, actions, callbacks, or registered Go functions.
- Interrupt running JavaScript in top-level scripts, condition checks, boolean actions, and switch-case actions.
- Check for cancellation between execution steps so no later condition or action starts after cancellation.
- Allow an already-running registered Go function to finish, then stop before continuing the rule chain.
- Return errors that match `context.Canceled` or `context.DeadlineExceeded` through `errors.Is`.
- Return the original rules-context pointer and preserve mutations made before interruption.
- Keep traces chronological, record the context error only on an interrupted condition, leave completed entries error-free, and omit conditions that never started.
- Keep cancellation state local to each run so concurrent executions on one runner remain isolated.
- Avoid new external dependencies and preserve all existing behavior, including under the race detector.
