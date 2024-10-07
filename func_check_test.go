package yabre

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckFunctionSignature(t *testing.T) {
	tests := []struct {
		name        string
		fn          any
		argTypes    []reflect.Type
		returnTypes []reflect.Type
		expectError bool
	}{
		{
			name:        "Matching function",
			fn:          func(a int, b string) (float64, error) { return 0, nil },
			argTypes:    []reflect.Type{reflect.TypeOf(0), reflect.TypeOf("")},
			returnTypes: []reflect.Type{reflect.TypeOf(0.0), reflect.TypeOf((*error)(nil)).Elem()},
			expectError: true,
		},
		{
			name:        "Non-function",
			fn:          "not a function",
			argTypes:    []reflect.Type{},
			returnTypes: []reflect.Type{},
			expectError: false,
		},
		{
			name:        "Mismatched argument count",
			fn:          func(a int) {},
			argTypes:    []reflect.Type{reflect.TypeOf(0), reflect.TypeOf("")},
			returnTypes: []reflect.Type{},
			expectError: false,
		},
		{
			name:        "Mismatched return count",
			fn:          func() (int, string) { return 0, "" },
			argTypes:    []reflect.Type{},
			returnTypes: []reflect.Type{reflect.TypeOf(0)},
			expectError: false,
		},
		{
			name:        "Mismatched argument type",
			fn:          func(a float64) {},
			argTypes:    []reflect.Type{reflect.TypeOf(0)},
			returnTypes: []reflect.Type{},
			expectError: false,
		},
		{
			name:        "Mismatched return type",
			fn:          func() int { return 0 },
			argTypes:    []reflect.Type{},
			returnTypes: []reflect.Type{reflect.TypeOf("")},
			expectError: false,
		},
		{
			name:        "Empty function",
			fn:          func() {},
			argTypes:    []reflect.Type{},
			returnTypes: []reflect.Type{},
			expectError: true,
		},
		{
			name:        "Assignable types",
			fn:          func(a int64) uint64 { return 0 },
			argTypes:    []reflect.Type{reflect.TypeOf(int64(0))},
			returnTypes: []reflect.Type{reflect.TypeOf(uint64(0))},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckFunctionSignature(tt.fn, tt.argTypes, tt.returnTypes)
			assert.True(t, (err != nil) != tt.expectError, "CheckFunctionSignature() error = %v, expectError %v", err, tt.expectError)
		})
	}
}

func TestCheckVariadicAnySignature(t *testing.T) {
	tests := []struct {
		name        string
		fn          any
		expectMatch bool
		expectError bool
	}{
		{
			name:        "Matching function",
			fn:          func(args ...any) (any, error) { return nil, nil },
			expectMatch: true,
			expectError: false,
		},
		{
			name:        "Non-function",
			fn:          "not a function",
			expectMatch: false,
			expectError: true,
		},
		{
			name:        "Non-variadic function",
			fn:          func(a any) (any, error) { return nil, nil },
			expectMatch: false,
			expectError: false,
		},
		{
			name:        "Wrong argument type",
			fn:          func(args ...int) (any, error) { return nil, nil },
			expectMatch: false,
			expectError: false,
		},
		{
			name:        "Wrong number of return values",
			fn:          func(args ...any) any { return nil },
			expectMatch: false,
			expectError: false,
		},
		{
			name:        "Wrong first return type",
			fn:          func(args ...any) (int, error) { return 0, nil },
			expectMatch: false,
			expectError: false,
		},
		{
			name:        "Wrong second return type",
			fn:          func(args ...any) (any, string) { return nil, "" },
			expectMatch: false,
			expectError: false,
		},
		{
			name:        "Extra arguments",
			fn:          func(prefix string, args ...any) (any, error) { return nil, nil },
			expectMatch: false,
			expectError: false,
		},
		{
			name:        "Empty variadic function",
			fn:          func(...any) (any, error) { return nil, nil },
			expectMatch: true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := checkVariadicAnySignature(tt.fn)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.expectMatch, match)
		})
	}
}
