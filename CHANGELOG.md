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