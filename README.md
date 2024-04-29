# Business Rules Engine (BRE)

A flexible and extensible Business Rules Engine (BRE) implemented in Go. This engine allows you to define and execute complex business rules using a YAML-based rules definition file.

## Features

- Define business rules using a declarative YAML syntax
- Execute rules based on specified conditions and actions
- Inject custom Go functions to extend the functionality of the rules engine
- Provide a debug callback function to log and monitor the execution of rules
- Terminate rule execution based on specific conditions
- Update the context during rule execution to store and manipulate data

## Usage

To use the Business Rules Engine (BRE) module in your Go project, you can import it using Go Modules:

```go
import "github.com/aleybovich/yabre"
```

1. Define your business rules in a YAML file. Here's an example (more below):

   ```yaml
   conditions:
     weight_less_500:
       description: Check if a condition is met
       check: |
         function check_condition() {
           return context.Weight < 500;
         }
       true:
         description: Perform an action if the condition is true
         action:
           function: |
             function check_condition_true() {
               context.Result = "Condition met";
             }
         terminate: true
   ```

2. Create a context struct that holds the necessary data for your rules:

   ```go
   type MyContext struct {
       Value  string `json:"value"`
       Result string `json:"result"`
   }
   ```

3. Initialize the rules runner with your YAML file and context:

   ```go
   context := MyContext{Value: "Valid"}
   runner, err := yabre.NewRulesRunnerFromYaml("rules.yaml", &context)
   if err != nil {
       // Handle the error
   }
   ```

4. Execute the rules:

   ```go
   updatedContext, err := runner.RunRules(&context, "check_condition")
   if err != nil {
       // Handle the error
   }
   ```

5. Access the updated context to retrieve the results:

   ```go
   fmt.Println(updatedContext.Result)
   ```

## Building the YAML Rules File

The YAML rules file defines the conditions and actions that make up your business rules. Here's a guide on how to structure your YAML file:

```yaml
conditions:
  condition_name:
    description: A brief description of the condition
    check: |
      function condition_name() {
        // JavaScript function that checks the condition
        // Return true if the condition is met, false otherwise
        // You can access the context using the 'context' object
      }
    true:
      description: A brief description of the action to perform if the condition is true
      action:
        function: |
          function condition_name_true() {
            // JavaScript function to execute if the condition is true
            // You can modify the context here
          }
      next: next_condition_name # Optional: The name of the next condition to evaluate
      terminate: true # Optional: Set to true to terminate rule execution
    false:
      description: A brief description of the action to perform if the condition is false
      action:
        function: |
          function condition_name_false() {
            // JavaScript function to execute if the condition is false
            // You can modify the context here
          }
      next: next_condition_name # Optional: The name of the next condition to evaluate
      terminate: true # Optional: Set to true to terminate rule execution
```

### Key Components:

- `conditions`: The top-level key that contains all the conditions.
- `condition_name`: A unique name for each condition.
- `description`: A brief description of the condition or action.
- `check`: A JavaScript function that evaluates the condition. It should return `true` if the condition is met, and `false` otherwise. You can access the context using the `context` object. 
- `true`: The action to perform if the condition evaluates to `true`.
  - `action`: A JavaScript function to execute if the condition is true. You can modify the context here.
  - `next`: (Optional) The name of the next condition to evaluate after executing the action.
  - `terminate`: (Optional) Set to `true` to terminate rule execution after executing the action.
- `false`: The action to perform if the condition evaluates to `false`. It follows the same structure as `true`.

### Naming Conventions

Condition name should follow YAML standards

Javascript function names should follow the following convention:
- for a `check` function, its name should match `condition_name`, e.g. `weight_less_500_g`
- for a `true` `action` function, its name should match `condition_name_true`, e.g. `weight_less_500_g_true`
- for a `false` `action` function, its name shoul match `condition_name_true`, e.g. `weight_less_500_g_false`

You can define multiple conditions within the `conditions` block. The engine will evaluate the conditions starting from the specified `startCondition` when calling `RunRules`.

Note that the JavaScript functions defined in the YAML file have access to the `context` object, which allows you to read and modify the context data during rule execution.


## Extending the Engine

You can extend the functionality of the rules engine by injecting custom Go functions using the `WithGoFunction` option. Here's an example:

```go
add := func(args ...interface{}) (interface{}, error) {
    a := args[0].(int64)
    b := args[1].(int64)
    return a + b, nil
}

runner, err := yabre.NewRulesRunnerFromYaml("rules.yaml", &context, yabre.WithGoFunction("add", add))
```

In your YAML rules file, you can then use the `add` function:

```yaml
conditions:
  check_sum:
    description: Check the sum of two numbers
    check: |
      function check_sum() {
        const result = add(2, 3);
        return result === 5;
      }
    true:
      description: Sum is correct
      terminate: true
```

## Debugging

You can provide a debug callback function to log and monitor the execution of rules using the `WithDebugCallback` option:

```go
runner, err := yabre.NewRulesRunnerFromYaml("rules.yaml", &context, yabre.WithDebugCallback(
    func(ctx MyContext, data interface{}) {
        fmt.Printf("Debug: %v\n", data)
    }))
```

In your YAML rules file, you can use the `debug` function to log messages:

```yaml
conditions:
  check_debug:
    description: Check if debug function is called
    check: |
      function check_debug() {
        debug(context, "Debug function called");
        return true;
      }
    true:
      terminate: true
```

## License

This project is licensed under the [MIT License](LICENSE).