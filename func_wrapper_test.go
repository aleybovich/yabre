package yabre

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGoFuncWrapper(t *testing.T) {
	t.Run("One argument function with error", func(t *testing.T) {
		f := func(a int) (interface{}, error) {
			return fmt.Sprintf("Got %d", a), nil
		}
		wrapped := goFuncWrapper(f)
		result, err := wrapped(42)
		assert.NoError(t, err)
		assert.Equal(t, "Got 42", result)
	})

	t.Run("One argument function without error", func(t *testing.T) {
		f := func(a int) interface{} {
			return fmt.Sprintf("Got %d", a)
		}
		wrapped := goFuncWrapper(f)
		result, err := wrapped(42)
		assert.NoError(t, err)
		assert.Equal(t, "Got 42", result)
	})

	t.Run("Three argument function", func(t *testing.T) {
		f := func(a string, b int, c float64) interface{} {
			return fmt.Sprintf("Got %s, %d, %.2f", a, b, c)
		}
		wrapped := goFuncWrapper(f)
		result, err := wrapped("hello", 42, 3.14)
		assert.NoError(t, err)
		assert.Equal(t, "Got hello, 42, 3.14", result)
	})

	t.Run("Function returning error", func(t *testing.T) {
		f := func(a int) (interface{}, error) {
			if a < 0 {
				return nil, errors.New("negative number")
			}
			return fmt.Sprintf("Got %d", a), nil
		}
		wrapped := goFuncWrapper(f)
		_, err := wrapped(-1)
		assert.EqualError(t, err, "negative number")
	})

	t.Run("Incorrect number of arguments", func(t *testing.T) {
		f := func(a int, b string) interface{} {
			return 0
		}
		wrapped := goFuncWrapper(f)
		_, err := wrapped(42)
		assert.EqualError(t, err, "expected 2 arguments, got 1")
	})

	t.Run("Incorrect argument type", func(t *testing.T) {
		f := func(a int) interface{} {
			return ""
		}
		wrapped := goFuncWrapper(f)
		_, err := wrapped("not an int")
		assert.EqualError(t, err, "argument 1 must be 'int' but received 'string'")
	})

	t.Run("Numeric type conversion", func(t *testing.T) {
		f := func(a, b float64) float64 {
			return a + b
		}
		wrapped := goFuncWrapper(f)
		result, err := wrapped(3, 4.5) // int and float64
		assert.NoError(t, err)
		assert.InDelta(t, 7.5, result, 0.0001)
	})

	t.Run("Non-function input", func(t *testing.T) {
		wrapped := goFuncWrapper(42)
		_, err := wrapped()
		assert.EqualError(t, err, "not a function")
	})

	t.Run("Function with incorrect return type", func(t *testing.T) {
		f := func() (int, int) {
			return 42, 24
		}
		wrapped := goFuncWrapper(f)
		_, err := wrapped()
		assert.EqualError(t, err, "second return value must be error, got int")
	})

	t.Run("Inserting into map", func(t *testing.T) {
		goFunctions := make(map[string]func(...interface{}) (interface{}, error))

		f1 := func(a, b int) interface{} {
			return a + b
		}

		f2 := func(a, b int) (interface{}, error) {
			if a < 0 || b < 0 {
				return nil, errors.New("negative numbers not allowed")
			}
			return a + b, nil
		}

		goFunctions["add"] = goFuncWrapper(f1)
		goFunctions["safeAdd"] = goFuncWrapper(f2)

		result1, err1 := goFunctions["add"](5, 3)
		assert.NoError(t, err1)
		assert.Equal(t, 8, result1)

		result2, err2 := goFunctions["safeAdd"](5, 3)
		assert.NoError(t, err2)
		assert.Equal(t, 8, result2)

		_, err3 := goFunctions["safeAdd"](-1, 3)
		assert.EqualError(t, err3, "negative numbers not allowed")
	})
}
