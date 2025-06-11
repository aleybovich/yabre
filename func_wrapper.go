package yabre

import (
	"errors"
	"fmt"
	"reflect"
)

// goFuncWrapper takes any function and returns a wrapper function with the signature `func(...any) (any, error)`.
// It dynamically checks and converts input arguments, calls the original function, and handles its return values.
// The wrapper supports functions with various argument types and either one or two return values (second must be `error`).
// It performs type checking, allows numeric type conversions, and provides detailed error messages for mismatches.
func goFuncWrapper(f any) func(...any) (any, error) {
	return func(args ...any) (res any, err error) {
		// Recover from panics and convert them to errors
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic recovered: %v", r)
			}
		}()

		fValue := reflect.ValueOf(f)
		fType := fValue.Type()

		if fType.Kind() != reflect.Func {
			return nil, errors.New("not a function")
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
		for i := range args {
			var expectedType reflect.Type
			if isVariadic && i >= numIn-1 {
				expectedType = fType.In(numIn - 1).Elem()
			} else {
				expectedType = fType.In(i)
			}

			if args[i] == nil {
				// Handle nil arguments - create a zero value of the expected type
				in[i] = reflect.Zero(expectedType)
			} else {
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
		}

		results := fValue.Call(in)

		switch len(results) {
		case 1:
			return results[0].Interface(), nil
		case 2:
			var returnErr error
			if results[1].Kind() == reflect.Interface && !results[1].IsNil() {
				errVal := results[1].Interface()
				if e, ok := errVal.(error); ok {
					returnErr = e
				} else {
					return nil, fmt.Errorf("second return value must be error, got %T", errVal)
				}
			} else if results[1].Kind() != reflect.Interface {
				return nil, fmt.Errorf("second return value must be error, got %v", results[1].Kind())
			}
			return results[0].Interface(), returnErr
		default:
			return nil, errors.New("function must return (any) or (any, error)")
		}
	}
}
