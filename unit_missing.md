# condition.go
## Proposed test
- Test Decision.UnmarshalYAML with valid YAML containing next field - `TestDecision_UnmarshalYAML_ValidNext`
- Test Decision.UnmarshalYAML with valid YAML containing terminate field - `TestDecision_UnmarshalYAML_ValidTerminate`
- Test Decision.UnmarshalYAML with error when both next and terminate are set - `TestDecision_UnmarshalYAML_ErrorBothNextAndTerminate`
- Test Decision.UnmarshalYAML with invalid YAML structure - `TestDecision_UnmarshalYAML_InvalidYAML`
- Test runCondition with check function not found error - `TestRunCondition_CheckFunctionNotFound`
- Test runCondition with check function execution error - `TestRunCondition_CheckFunctionExecutionError`
- Test runCondition with true branch execution - `TestRunCondition_TrueBranchExecution`
- Test runCondition with false branch execution - `TestRunCondition_FalseBranchExecution`
- Test runCondition with null decision handling - `TestRunCondition_NullDecisionHandling`
- Test runAction with action function not found error - `TestRunAction_ActionFunctionNotFound`
- Test runAction with action function execution error - `TestRunAction_ActionFunctionExecutionError`
- Test runAction with next condition not found error - `TestRunAction_NextConditionNotFound`
- Test runAction with terminate flag behavior - `TestRunAction_TerminateFlagBehavior`
- Test findConditionByName with existing condition - `TestFindConditionByName_ExistingCondition`
- Test findConditionByName with condition not found - `TestFindConditionByName_ConditionNotFound`

# func_wrapper.go
## Proposed test
- Test goFuncWrapper with variadic functions and empty arguments - `TestGoFuncWrapper/Variadic_function_with_empty_arguments`
- Test goFuncWrapper with variadic functions and multiple arguments - `TestGoFuncWrapper/Variadic_function_with_multiple_arguments`
- Test goFuncWrapper with functions returning no values - `TestGoFuncWrapper/Function_returning_no_values`
- Test goFuncWrapper with functions returning more than 2 values - `TestGoFuncWrapper/Function_returning_more_than_2_values`
- Test goFuncWrapper with nil arguments - `TestGoFuncWrapper/Nil_argument_handling`
- Test goFuncWrapper with panic recovery - `TestGoFuncWrapper/Panic_recovery`
- Test goFuncWrapper with interface{} to complex type conversions (structs, slices, maps) - `TestGoFuncWrapper/Interface_to_complex_type_conversions`

# mermaid.go
## Proposed test
- Test ExportMermaid with invalid YAML input
- Test ExportMermaid with empty YAML
- Test ExportMermaid with malformed rules structure
- Test ExportMermaidFromLibrary with rule loading failure
- Test ExportMermaidFromLibrary with invalid rule name
- Test mermaid generation with conditions having empty descriptions
- Test mermaid generation with circular dependencies
- Test mermaid generation with special characters in names/descriptions
- Test mermaid generation with complex nested decision trees

# rules.go
## Proposed test
- Test Rules.UnmarshalYAML with multiple default conditions error - `TestRules_UnmarshalYAML_MultipleDefaultConditions`
- Test Rules.UnmarshalYAML with missing required fields - `TestRules_UnmarshalYAML_MissingRequiredFields`
- Test Rules.UnmarshalYAML with complex nested conditions - `TestRules_UnmarshalYAML_ComplexNestedConditions`
- Test loadRulesFromYaml with invalid YAML error - `TestLoadRulesFromYaml_InvalidYAMLError`
- Test loadRulesFromYaml with empty YAML - `TestLoadRulesFromYaml_EmptyYAML`
- Test addJsFunctions with script injection errors - `TestAddJsFunctions_ScriptInjectionErrors`
- Test addJsFunctions with invalid JavaScript syntax - `TestAddJsFunctions_InvalidJavaScriptSyntax`
- Test addJsFunctions with missing check functions - `TestAddJsFunctions_MissingCheckFunctions`
- Test injectJSFunction with various function name patterns - `TestInjectJSFunction_VariousFunctionNamePatterns`
- Test injectJSFunction with arrow functions - `TestInjectJSFunction_ArrowFunctions`
- Test injectJSFunction with anonymous functions - `TestInjectJSFunction_AnonymousFunctions`
- Test injectJSFunction with invalid function code - `TestInjectJSFunction_InvalidFunctionCode`

# rules_library.go
## Proposed test
- Test LoadRules with circular dependencies
- Test LoadRules with missing dependencies
- Test LoadRules with file read errors
- Test LoadRules with corrupt YAML files
- Test mergeRules with duplicate condition names
- Test mergeRules with complex script merging scenarios
- Test resolveDependencies with complex dependency graphs
- Test resolveDependencies with missing rule sets
- Test scanFiles with files without names
- Test scanFiles with duplicate rule names
- Test scanFiles with file system permission errors
- Test scanFiles with symbolic links

# runner.go
## Proposed test
- Test NewRulesRunnerFromLibrary with invalid options
- Test NewRulesRunnerFromLibrary with library load errors
- Test RunRules with no default condition error
- Test RunRules with JavaScript runtime errors
- Test RunRules with context export failures
- Test RunRules with nil context
- Test RunRules with empty rules
- Test WithGoFunction with invalid function signatures
- Test WithGoFunction with nil functions
- Test WithDebugCallback with nil callback
- Test WithDecisionCallback with callback errors
- Test concurrent RunRules execution for thread safety