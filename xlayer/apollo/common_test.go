package apollo

import (
	"testing"
)

// TestGetConfigValueFromType tests parsing various types
func TestGetConfigValueFromType(t *testing.T) {
	svc := &ApolloService{}

	t.Run("uint64", func(t *testing.T) {
		cv, err := svc.GetConfigValueFromType(uint64(12345))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.typ != TypeU64 {
			t.Errorf("type = %v, want TypeU64", cv.typ)
		}
		if cv.u64 != 12345 {
			t.Errorf("value = %v, want 12345", cv.u64)
		}
	})

	t.Run("int64", func(t *testing.T) {
		cv, err := svc.GetConfigValueFromType(int64(-999))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.typ != TypeI64 {
			t.Errorf("type = %v, want TypeI64", cv.typ)
		}
		if cv.i64 != -999 {
			t.Errorf("value = %v, want -999", cv.i64)
		}
	})

	t.Run("int to int64", func(t *testing.T) {
		cv, err := svc.GetConfigValueFromType(int(42))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.typ != TypeI64 {
			t.Errorf("type = %v, want TypeI64", cv.typ)
		}
		if cv.i64 != 42 {
			t.Errorf("value = %v, want 42", cv.i64)
		}
	})

	t.Run("bool true", func(t *testing.T) {
		cv, err := svc.GetConfigValueFromType(true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.typ != TypeBool {
			t.Errorf("type = %v, want TypeBool", cv.typ)
		}
		if cv.boolVal != true {
			t.Errorf("value = %v, want true", cv.boolVal)
		}
	})

	t.Run("bool false", func(t *testing.T) {
		cv, err := svc.GetConfigValueFromType(false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.typ != TypeBool {
			t.Errorf("type = %v, want TypeBool", cv.typ)
		}
		if cv.boolVal != false {
			t.Errorf("value = %v, want false", cv.boolVal)
		}
	})

	t.Run("float64", func(t *testing.T) {
		cv, err := svc.GetConfigValueFromType(float64(3.14159))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.typ != TypeF64 {
			t.Errorf("type = %v, want TypeF64", cv.typ)
		}
		if cv.f64 != 3.14159 {
			t.Errorf("value = %v, want 3.14159", cv.f64)
		}
	})

	t.Run("string as string", func(t *testing.T) {
		cv, err := svc.GetConfigValueFromType("hello world")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.typ != TypeString {
			t.Errorf("type = %v, want TypeString", cv.typ)
		}
		if cv.str != "hello world" {
			t.Errorf("value = %v, want 'hello world'", cv.str)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		cv, err := svc.GetConfigValueFromType("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.typ != TypeString {
			t.Errorf("type = %v, want TypeString", cv.typ)
		}
		if cv.str != "" {
			t.Errorf("value = %q, want empty string", cv.str)
		}
	})

	t.Run("simple array", func(t *testing.T) {
		input := []int64{1, 2, 3}
		cv, err := svc.GetConfigValueFromType(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.typ != TypeArray {
			t.Errorf("type = %v, want TypeArray", cv.typ)
		}
		if len(cv.array) != 3 {
			t.Fatalf("array length = %v, want 3", len(cv.array))
		}
		for i, expected := range []int64{1, 2, 3} {
			if cv.array[i].typ != TypeI64 {
				t.Errorf("array[%d].typ = %v, want TypeI64", i, cv.array[i].typ)
			}
			if cv.array[i].i64 != expected {
				t.Errorf("array[%d].i64 = %v, want %v", i, cv.array[i].i64, expected)
			}
		}
	})

	t.Run("nested array", func(t *testing.T) {
		input := [][]int64{
			{1, 2},
			{3, 4},
		}
		cv, err := svc.GetConfigValueFromType(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.typ != TypeArray {
			t.Errorf("type = %v, want TypeArray", cv.typ)
		}
		if len(cv.array) != 2 {
			t.Fatalf("outer array length = %v, want 2", len(cv.array))
		}

		// Check nested arrays
		for i := 0; i < 2; i++ {
			if cv.array[i].typ != TypeArray {
				t.Errorf("array[%d].typ = %v, want TypeArray", i, cv.array[i].typ)
			}
			if len(cv.array[i].array) != 2 {
				t.Errorf("array[%d] length = %v, want 2", i, len(cv.array[i].array))
			}
		}
	})

	t.Run("empty array", func(t *testing.T) {
		input := []interface{}{}
		cv, err := svc.GetConfigValueFromType(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.typ != TypeArray {
			t.Errorf("type = %v, want TypeArray", cv.typ)
		}
		if len(cv.array) != 0 {
			t.Errorf("array length = %v, want 0", len(cv.array))
		}
	})

	t.Run("unsupported type converts to string", func(t *testing.T) {
		type customStruct struct{ value int }
		input := customStruct{value: 42}
		cv, err := svc.GetConfigValueFromType(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Default case converts to string representation
		if cv.typ != TypeString {
			t.Errorf("type = %v, want TypeString", cv.typ)
		}
		if cv.str == "" {
			t.Error("expected non-empty string representation")
		}
	})

	t.Run("nil value", func(t *testing.T) {
		cv, err := svc.GetConfigValueFromType(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// nil converts to string
		if cv.typ != TypeString {
			t.Errorf("type = %v, want TypeString", cv.typ)
		}
	})
}

// TestGetConfigValueFromType_EdgeCases tests edge cases
func TestGetConfigValueFromType_EdgeCases(t *testing.T) {
	svc := &ApolloService{}

	t.Run("max uint64", func(t *testing.T) {
		cv, err := svc.GetConfigValueFromType(uint64(18446744073709551615))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.u64 != 18446744073709551615 {
			t.Errorf("value = %v, want max uint64", cv.u64)
		}
	})

	t.Run("min int64", func(t *testing.T) {
		cv, err := svc.GetConfigValueFromType(int64(-9223372036854775808))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cv.i64 != -9223372036854775808 {
			t.Errorf("value = %v, want min int64", cv.i64)
		}
	})

	t.Run("zero values", func(t *testing.T) {
		tests := []struct {
			name  string
			input interface{}
			typ   ConfigValueType
		}{
			{"zero uint64", uint64(0), TypeU64},
			{"zero int64", int64(0), TypeI64},
			{"zero int", int(0), TypeI64},
			{"zero float64", float64(0), TypeF64},
			{"false bool", false, TypeBool},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cv, err := svc.GetConfigValueFromType(tt.input)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if cv.typ != tt.typ {
					t.Errorf("type = %v, want %v", cv.typ, tt.typ)
				}
			})
		}
	})
}
