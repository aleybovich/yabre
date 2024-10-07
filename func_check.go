package yabre

import (
	"fmt"
	"reflect"
)

// CheckFunctionSignature checks if the given function matches the specified argument and return types
func CheckFunctionSignature(fn any, argTypes []reflect.Type, returnTypes []reflect.Type) error {
	fnType := reflect.TypeOf(fn)

	if fnType.Kind() != reflect.Func {
		return fmt.Errorf("not a function")
	}

	if fnType.NumIn() != len(argTypes) || fnType.NumOut() != len(returnTypes) {
		return fmt.Errorf("expected %d arguments and %d return values, got %d and %d", len(argTypes), len(returnTypes), fnType.NumIn(), fnType.NumOut())
	}

	for i := 0; i < fnType.NumIn(); i++ {
		if !fnType.In(i).AssignableTo(argTypes[i]) {
			return fmt.Errorf("argument %d must be '%v' but received '%v'", i+1, argTypes[i], fnType.In(i))
		}
	}

	for i := 0; i < fnType.NumOut(); i++ {
		if !fnType.Out(i).AssignableTo(returnTypes[i]) {
			return fmt.Errorf("return value %d must be '%v' but received '%v'", i+1, returnTypes[i], fnType.Out(i))
		}
	}

	return nil
}

// checkVariadicAnySignature checks if the given function matches the signature `func(args ...any) (any, error)`
func checkVariadicAnySignature(fn any) (bool, error) {
	fnType := reflect.TypeOf(fn)

	if fnType.Kind() != reflect.Func {
		return false, fmt.Errorf("not a function")
	}

	if fnType.NumIn() != 1 || fnType.NumOut() != 2 {
		return false, nil
	}

	if !fnType.IsVariadic() || fnType.In(0) != reflect.TypeOf([]any{}) {
		return false, nil
	}

	if fnType.Out(0) != reflect.TypeOf((*any)(nil)).Elem() || fnType.Out(1) != reflect.TypeOf((*error)(nil)).Elem() {
		return false, nil
	}

	return true, nil
}
