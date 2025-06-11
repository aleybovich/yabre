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

	t.Run("Variadic function with empty arguments", func(t *testing.T) {
		f := func(prefix string, args ...int) interface{} {
			return fmt.Sprintf("%s: %v", prefix, args)
		}
		wrapped := goFuncWrapper(f)
		result, err := wrapped("numbers")
		assert.NoError(t, err)
		assert.Equal(t, "numbers: []", result)
	})

	t.Run("Variadic function with multiple arguments", func(t *testing.T) {
		f := func(prefix string, args ...int) interface{} {
			sum := 0
			for _, v := range args {
				sum += v
			}
			return fmt.Sprintf("%s: sum=%d", prefix, sum)
		}
		wrapped := goFuncWrapper(f)
		result, err := wrapped("total", 1, 2, 3, 4, 5)
		assert.NoError(t, err)
		assert.Equal(t, "total: sum=15", result)
	})

	t.Run("Function returning no values", func(t *testing.T) {
		f := func() {
			// Function with no return values
		}
		wrapped := goFuncWrapper(f)
		_, err := wrapped()
		assert.EqualError(t, err, "function must return (any) or (any, error)")
	})

	t.Run("Function returning more than 2 values", func(t *testing.T) {
		f := func() (int, string, error) {
			return 42, "hello", nil
		}
		wrapped := goFuncWrapper(f)
		_, err := wrapped()
		assert.EqualError(t, err, "function must return (any) or (any, error)")
	})

	t.Run("Nil argument handling", func(t *testing.T) {
		f := func(a interface{}) interface{} {
			if a == nil {
				return "got nil"
			}
			return fmt.Sprintf("got %v", a)
		}
		wrapped := goFuncWrapper(f)
		result, err := wrapped(nil)
		assert.NoError(t, err)
		assert.Equal(t, "got nil", result)
	})

	t.Run("Panic recovery", func(t *testing.T) {
		f := func(a int) interface{} {
			if a == 0 {
				panic("division by zero")
			}
			return 10 / a
		}
		wrapped := goFuncWrapper(f)
		_, err := wrapped(0)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "panic recovered")
		assert.Contains(t, err.Error(), "division by zero")
	})

	t.Run("Interface to complex type conversions", func(t *testing.T) {
		// Struct conversion
		type TestStruct struct {
			Name  string
			Value int
		}
		f1 := func(data interface{}) interface{} {
			if s, ok := data.(map[string]interface{}); ok {
				return TestStruct{
					Name:  s["name"].(string),
					Value: int(s["value"].(float64)),
				}
			}
			return nil
		}
		wrapped1 := goFuncWrapper(f1)
		input := map[string]interface{}{"name": "test", "value": float64(42)}
		result1, err := wrapped1(input)
		assert.NoError(t, err)
		expected := TestStruct{Name: "test", Value: 42}
		assert.Equal(t, expected, result1)

		// Slice conversion
		f2 := func(data interface{}) interface{} {
			if slice, ok := data.([]interface{}); ok {
				result := make([]int, len(slice))
				for i, v := range slice {
					result[i] = int(v.(float64))
				}
				return result
			}
			return nil
		}
		wrapped2 := goFuncWrapper(f2)
		result2, err := wrapped2([]interface{}{float64(1), float64(2), float64(3)})
		assert.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, result2)

		// Map conversion
		f3 := func(data interface{}) interface{} {
			if m, ok := data.(map[string]interface{}); ok {
				result := make(map[string]string)
				for k, v := range m {
					result[k] = fmt.Sprintf("%v", v)
				}
				return result
			}
			return nil
		}
		wrapped3 := goFuncWrapper(f3)
		result3, err := wrapped3(map[string]interface{}{"a": 1, "b": "hello"})
		assert.NoError(t, err)
		expected3 := map[string]string{"a": "1", "b": "hello"}
		assert.Equal(t, expected3, result3)
	})
}
