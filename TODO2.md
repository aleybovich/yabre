# Problem 2: Error-handling transitions (`on_error`) with error classification

## Problem description

Yabre executes user-authored JavaScript, and in production most checks and actions are thin JavaScript wrappers around registered Go functions — so failures usually originate as Go errors (a gateway timeout, a validation failure, a database error) that surface through the JavaScript layer. Today any such failure aborts the entire run. Rule flows need the same thing state-machine engines provide (compare AWS Step Functions' `Catch`): route a failure to a remediation path defined in the rule set itself, while letting the host application decide which errors are recoverable and which are show-stoppers.

### YAML surface

`on_error` may appear at the rule-set level and/or on individual conditions:

```yaml
name: payment-rules
on_error: record_failure          # rule-set-level default handler

conditions:
  charge_card:
    default: true
    on_error: retry_via_backup    # condition-level override
    check: |
      function() { return chargeCard(context.Amount); }
    true:
      terminate: true
    false:
      next: charge_failed

  retry_via_backup:
    check: ...

  record_failure:
    check: ...
```

New fields: `OnError string` with YAML tag `on_error` on both `Rules` and `Condition`. Empty string means "not set".

### Error classification

- New exported type `ErrorDisposition` with constants `ErrorRoutable` and `ErrorFatal`; `ErrorRoutable` is the zero value.
- New runner option following the existing `WithOption` pattern:

  ```go
  func WithErrorClassifier[Context any](classifier func(err error) ErrorDisposition) WithOption[Context]
  ```

- Consulted only when a transfer would otherwise occur (qualifying error + resolved handler + transfer budget available), and before the handler name is looked up. `ErrorRoutable` → transfer; `ErrorFatal` → abort exactly as if no handler were configured (same error, same trace, no `lastError`, no transfer callback message), even if the handler name does not exist. No classifier registered → every qualifying error is routable.
- **Seamless Go-error integration.** The error the classifier receives must preserve the cause chain: when the failure originated from a registered Go function returning a non-nil error, `errors.Is`/`errors.As` against that original Go error must work. Pure JS `throw` → classifier receives the JS evaluation error. The same chain preservation applies to the error returned by an aborted run and to the failing condition's `TraceEntry.Error`.

### Runtime semantics

1. **Qualifying errors**: errors raised while executing a condition's JavaScript — the check function, or the matched true/false-branch or switch-case action — including exceptions originating from registered Go functions. **Not** qualifying (abort exactly as today): "check function not found" / "action function not found", a `next` reference to a missing condition, and script/function injection errors during setup.
2. **Handler resolution only at the failing condition**: its own `on_error` if non-empty, else the rule set's `on_error` if non-empty, else no handler. No fallback from condition-level to set-level. An error propagating up from a downstream condition must never trigger an ancestor's handler.
3. **Transfer sequence** (routable, budget available): decision callback with exactly `"Error in condition [%s], transferring to error handler [%s]"` (failing name, handler name) → define JS global `lastError` = `{conditionName, message}` where `message` equals the failing condition's trace-entry error message → evaluate the handler condition as a normal condition. On success the call returns nil error; context mutations made before the failure are preserved; the original chain is never resumed. `lastError` must not exist before the first transfer and stays defined for the rest of the run.
4. **At most one transfer per call.** Any later qualifying error — including inside the handler chain — aborts as an unhandled error does today. Self-handler (`on_error` naming its own condition) is legal; its second failure aborts.
5. **Missing handler at runtime** (routable transfer, name not in merged rule set): return an error whose message contains `on_error handler condition '<name>' not found`.
6. **Per-call isolation.** Transfer state is per execution, never on the runner. Sequential calls each get a fresh transfer budget; concurrent calls on one runner must not interfere; clean under `-race`.

### Tracing

- Failing condition's entry: identical to an unhandled failure today (same `Error` message, `Duration` = check + direct action only), even though the call returns nil after a successful transfer.
- Handler-chain entries appended after it, chronological, existing conventions.
- Already-completed ancestor entries keep nil `Error` and their pre-failure field values.
- Second failure: first failing entry keeps its original error; second failing entry records its own; the call returns an error identifying the second failure.
- Fatal classification leaves the trace exactly as an unhandled failure today.

### Library semantics

- Condition-level `on_error` travels with its condition through merges.
- The governing rule-set-level `on_error` is the loaded rule set's own; a dependency's rule-set-level value is ignored on merge.
- Identical behavior whether rules are loaded through `RulesLibrary` (incl. `require` dependencies) or constructed directly — rule-set-level `on_error` must survive library loading.

### Validation (`ValidateRules`)

- Rule-set-level `on_error` naming a missing condition → `SeverityError`, empty `ConditionName`, message containing `on_error references non-existent condition '<name>'`.
- Condition-level equivalent → `SeverityError` with `ConditionName` set, same message format.
- A condition referenced only via `on_error` (either level) is reachable — no "unreachable" warning.
- Cycle detection must not follow `on_error` edges; self-handler is valid and not a circular dependency.

### Constraints

- Mermaid export unchanged. No new external dependencies. Public API stays source-compatible. All existing tests keep passing, including under `go test -race`.

---

## Implementation plan

### 1. Data model — `rules.go`, `condition.go`

- `Rules`: add `OnError string \`yaml:"on_error,omitempty"\`` (omitempty so `ExportMermaidFromLibrary`'s re-marshal stays clean when unset).
- `Condition`: add `OnError string \`yaml:"on_error"\``.
- **`Rules.copy()` (rules.go:85) must carry `OnError`.** It rebuilds the struct field-by-field; missing this silently drops the set-level handler for every library-loaded rule set, because `LoadRules` always goes through `copy()`.
- `mergeRules` (rules_library.go:141): deliberately do **not** touch `target.OnError` / `source.OnError` — the loaded set's value governs; a dependency's set-level value is ignored by design. Condition-level values travel inside the `Condition` values already merged.
- No `UnmarshalYAML` changes needed — both custom unmarshallers use intermediate types that pick up new fields automatically.

### 2. Classification API — `runner.go`

- `type ErrorDisposition int`; `const ( ErrorRoutable ErrorDisposition = iota; ErrorFatal )`.
- Runner field `errorClassifier func(error) ErrorDisposition` + `WithErrorClassifier` option mirroring `WithDebugCallback`.

### 3. Per-run execution state — `condition.go`

- `type runState struct { transferred bool }`, allocated once in `executeRules`, threaded by pointer through `runCondition` → `runBoolCondition`/`runSwitchCondition` → `runAction` (all internal signatures — free to change). Mirrors the `traceCollector` pattern; nothing per-run is ever stored on `RulesRunner` (concurrency requirement).

### 4. Distinguishing direct action errors from downstream/structural errors — `condition.go`

`runAction` today returns three indistinguishable error classes: (a) the direct action invocation failed, (b) a downstream condition in the `next` chain failed, (c) structural ("action function not found"). Only (a) may trigger a transfer at *this* condition; (b) was already resolved at its own site; (c) always aborts.

- Introduce an internal marker: `type actionExecError struct{ error }` with `Unwrap`. In `runAction`, wrap only the direct invocation failure: `&actionExecError{fmt.Errorf("error running action: %w", err)}`.
- In `runBoolCondition`/`runSwitchCondition`, after `runAction` returns: `errors.As` for the marker. If present, strip it (use the inner error everywhere — trace entry, propagation) so ancestors only ever see plain wrapped errors and can never re-match the marker. If absent, propagate as today (no transfer attempt).
- Check-function failures need no marker — `runCondition` already knows the error is its own check's.

### 5. Transfer helper — `condition.go`

Single helper used from both the check-error site (in `runCondition`) and the action-error sites (in the two branch runners):

```go
// attemptTransfer returns (attempted, err). attempted=false → caller propagates original error as today.
func (rr *RulesRunner[Context]) attemptTransfer(vm, rules, failing *Condition, cause error, st *runState, tc *traceCollector) (bool, error)
```

Sequence inside:

1. If `st.transferred` → return `(false, nil)` (budget spent; caller propagates).
2. Resolve name: `failing.OnError`, else `rules.OnError`; empty → `(false, nil)`.
3. Classifier: if set and `ErrorFatal` → `(false, nil)` — happens **before** handler lookup per spec.
4. `st.transferred = true`.
5. `findConditionByName`; missing → `(true, fmt.Errorf("on_error handler condition '%s' not found", name))`.
6. `rr.decisionCallback("Error in condition [%s], transferring to error handler [%s]", failing.Name, name)`.
7. `vm.Set("lastError", map[string]any{"conditionName": failing.Name, "message": cause.Error()})` — goja exposes a `map[string]any` as a plain JS object; not setting it earlier keeps `typeof lastError === 'undefined'` before the first transfer.
8. Return `(true, rr.runCondition(vm, rules, handler, tc, st))` — handler chain is a normal chain; a second failure inside it propagates normally because `st.transferred` is now true.

Call-site ordering (critical for trace correctness): record the failing entry's `Duration` and `Error` **before** calling `attemptTransfer`, so the entry keeps the error and its duration excludes the handler chain.

### 6. Error-chain preservation (Go → JS → Go)

Verified in vendored goja (`runtime.go`): a reflect-wrapped Go function returning non-nil error makes goja `panic(r.NewGoError(err))`, a JS `GoError` object whose `value` property holds the original Go error; back in Go the failure is a `*goja.Exception`, which implements `Unwrap() error` returning that original error. Since every yabre wrap uses `%w` (`"error evaluating check function %s: %w"`, `"error running action: %w"`), `errors.Is/As` already reach the original Go error end-to-end. Rules for the implementation: keep `%w` in every new/existing wrap, pass the classifier the recorded (wrapped) cause, and never stringify an error into a new one.

### 7. Validation — `validate.go`

- `detectDanglingReferences`: add — if `rules.OnError != ""` and missing → `SeverityError` with `ConditionName: ""`, message `on_error references non-existent condition '<name>'`; same per condition with `ConditionName: name`.
- `detectUnreachableConditions`: mark `rules.OnError` and every `condition.OnError` as referenced.
- `detectConditionCycles` / `walkCondition`: **unchanged** — `on_error` edges are not followed (the one-transfer budget makes runtime loops impossible; a self-handler must not be reported as a cycle).
- Library `validateAll` picks all of this up automatically via `ValidateRules`.

### 8. Test plan (grading suite)

Runtime: transfer from a failing check; from a failing true/false action; from a failing switch-case action; Go-function-origin failure where the classifier proves `errors.Is` against a wrapped sentinel; classifier fatal (abort, no `lastError`, no callback message, trace = unhandled); classifier fatal with dangling handler name (original error returned); no classifier → routable default; condition-level overrides set-level (no fallback); second failure in handler chain aborts + both trace entries correct; self-handler double failure; missing handler at runtime (message check); structural errors don't transfer (missing check fn, missing action fn, dangling `next` — each with a handler configured); `lastError` undefined before transfer / correct fields in handler check, action, and downstream conditions; decision-callback exact message; context mutations before failure preserved; ancestors' trace entries nil-error while failing entry keeps its error; chronological order; downstream failure never triggers ancestor handler.

Isolation: two sequential runs on one runner both transfer (fresh budget each); concurrent runs under `-race`.

Library: set-level `on_error` survives `RulesLibrary` loading (catches a missed `copy()`); dependency's set-level value ignored while main's governs merged conditions; condition-level value travels from a dependency.

Validation: dangling set-level and condition-level refs (severity, `ConditionName`, message); handler-only condition not flagged unreachable (both levels); self-handler produces no cycle error; library init reports no spurious warnings for handler-only conditions.

Regression: entire existing suite green under `go test -race ./...`.

### Likely integration points a naive implementation misses

1. `Rules.copy()` not carrying `OnError` → set-level handler lost through the library path.
2. `detectUnreachableConditions` not updated → spurious warnings for handler-only conditions.
3. Transfer flag stored on `RulesRunner` → sequential second run loses its budget; races under concurrency.
4. Transfer attempted wherever an error is *seen* (e.g. top of `executeRules` or in `runAction`'s next-chain wrap) instead of where it *occurred* → ancestor entries wrongly carry errors, ancestor handlers wrongly fire for downstream failures.
5. Structural errors ("check function not found") routed to handlers.
6. Error chains broken by `%v` wrapping or `err.Error()` stringification → classifier can't `errors.Is` the Go sentinel.
