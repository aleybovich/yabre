[0.8.1]
- Fixed: `goFuncWrapper` now properly handles nil arguments without panic
- Fixed: `goFuncWrapper` now recovers from panics and converts them to errors
- Added: Comprehensive unit tests for `condition.go` (15 test functions)
- Added: Comprehensive unit tests for `rules.go` (12 test functions)
- Added: Additional unit tests for `func_wrapper.go` covering edge cases (7 new test cases)
- Added: Comprehensive edge case tests for the business rules engine in `runner_edge_cases_test.go`
- Improved: Test coverage for variadic functions, nil handling, panic recovery, and complex type conversions
- Improved: Test coverage for various rule engine scenarios including circular references, error propagation, concurrent execution, and performance with large datasets

[0.8.0]
- Support for modular rule sets through a library system
- Ability to organize rules across multiple files with dependencies

[0.7.0]
- Changed: hnhanced `WithGoFunction` with support of any function signature; `goFuncWrapper` is now internal which is a breaking change

[0.6.0]
- Added: `GoFuncWrapper` wrapper function that allows extending BRE with strongly typed functions
- Changed: updated README to reflect the improvement

[0.5.0]
- Added: custom reusable js functions maybe defined in `scripts` section of a rules yaml file
- Changed: injectable debug function arguments from `(context, interface{})` to `(...interface{})`
- Changed: updated unit tests

[0.4.0]
- Changed: fixed module name

[0.3.0]
- Changed: Rules runner constructor accepts yaml byte array instead of yaml file name
- Changed: Updated README to include a section about Mermaid conversion functionality
- Changed: more verbose decision callbacks

[0.2.0]
- Added: `ExportMermaid` function accepts business rules in YAML and returns Mermaid code to display the rules in a flow disagram
- Added: `WithDecisionCallback` option in RulesRunner constructor; when provided, used to receive the decision steps from rules engine; to be used for rules troubleshooting or audit 
- Added: support for anonymous or custom named javascript functions 
- Changed: code refactoring/cleanup

[0.1.0] 
- Added: business rules engine