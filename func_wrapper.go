package yabre

import (
	"fmt"
	"reflect"
)

// goFuncWrapper takes any function and returns a wrapper function with the signature `func(...any) (any, error)`.
// It dynamically checks and converts input arguments, calls the original function, and handles its return values.
// The wrapper supports functions with various argument types and either one or two return values (second must be `error`).
// It performs type checking, allows numeric type conversions, and provides detailed error messages for mismatches.
func goFuncWrapper(f any) func(...any) (any, error) {
	return func(args ...any) (any, error) {
		fValue := reflect.ValueOf(f)
		fType := fValue.Type()

		if fType.Kind() != reflect.Func {
			return nil, fmt.Errorf("not a function")
		}

		numIn := fType.NumIn()
		isVariadic := fType.IsVariadic()

		if !isVariadic && numIn != len(args) {
			return nil, fmt.Errorf("expected %d arguments, got %d", fType.NumIn(), len(args))
		}

		if isVariadic && len(args) < numIn-1 {
			return nil, fmt.Errorf("expected at least %d arguments, got %d", numIn-1, len(args))
		}

		in := make([]reflect.Value, len(args))
		for i := 0; i < len(args); i++ {
			var expectedType reflect.Type
			if isVariadic && i >= numIn-1 {
				expectedType = fType.In(numIn - 1).Elem()
			} else {
				expectedType = fType.In(i)
			}

			receivedType := reflect.TypeOf(args[i])

			if receivedType.AssignableTo(expectedType) {
				in[i] = reflect.ValueOf(args[i])
			} else if receivedType.ConvertibleTo(expectedType) {
				// Handle numeric type conversions
				in[i] = reflect.ValueOf(args[i]).Convert(expectedType)
			} else {
				return nil, fmt.Errorf("argument %d must be '%v' but received '%v'", i+1, expectedType, receivedType)
			}
		}

		result := fValue.Call(in)

		switch len(result) {
		case 1:
			return result[0].Interface(), nil
		case 2:
			var err error
			if result[1].Kind() == reflect.Interface && !result[1].IsNil() {
				errVal := result[1].Interface()
				if e, ok := errVal.(error); ok {
					err = e
				} else {
					return nil, fmt.Errorf("second return value must be error, got %T", errVal)
				}
			} else if result[1].Kind() != reflect.Interface {
				return nil, fmt.Errorf("second return value must be error, got %v", result[1].Kind())
			}
			return result[0].Interface(), err
		default:
			return nil, fmt.Errorf("function must return (any) or (any, error)")
		}
	}
}
