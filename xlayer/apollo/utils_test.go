package apollo

import (
	"reflect"
	"testing"
)

// TestGetNamespacePrefix tests namespace prefix extraction
func TestGetNamespacePrefix(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		want      string
		wantErr   bool
	}{
		{
			name:      "simple namespace",
			namespace: "jsonrpc",
			want:      "jsonrpc",
			wantErr:   false,
		},
		{
			name:      "namespace with suffix",
			namespace: "opgeth-l2gaspricer",
			want:      "opgeth",
			wantErr:   false,
		},
		{
			name:      "namespace with multiple parts",
			namespace: "opnode-sequencer-config",
			want:      "opnode",
			wantErr:   false,
		},
		{
			name:      "empty string",
			namespace: "",
			want:      "",
			wantErr:   false, // Note: Current implementation returns empty string, not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNamespacePrefix(tt.namespace)
			if got != tt.want {
				t.Errorf("getNamespacePrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestConvertArrayToSlice tests array to slice conversions
func TestConvertArrayToSlice(t *testing.T) {
	t.Run("int64 slice", func(t *testing.T) {
		cv := ConfigValue{
			typ: TypeArray,
			array: []ConfigValue{
				{typ: TypeI64, i64: 1},
				{typ: TypeI64, i64: 2},
				{typ: TypeI64, i64: 3},
			},
		}

		targetType := reflect.TypeOf([]int64{})
		result, ok := convertArrayToSlice(cv, targetType)
		if !ok {
			t.Fatal("convertArrayToSlice failed")
		}

		got, ok := result.([]int64)
		if !ok {
			t.Fatalf("result type = %T, want []int64", result)
		}

		want := []int64{1, 2, 3}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("uint64 slice", func(t *testing.T) {
		cv := ConfigValue{
			typ: TypeArray,
			array: []ConfigValue{
				{typ: TypeU64, u64: 10},
				{typ: TypeU64, u64: 20},
			},
		}

		targetType := reflect.TypeOf([]uint64{})
		result, ok := convertArrayToSlice(cv, targetType)
		if !ok {
			t.Fatal("convertArrayToSlice failed")
		}

		got := result.([]uint64)
		want := []uint64{10, 20}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("int slice", func(t *testing.T) {
		cv := ConfigValue{
			typ: TypeArray,
			array: []ConfigValue{
				{typ: TypeI64, i64: 5},
				{typ: TypeI64, i64: 10},
			},
		}

		targetType := reflect.TypeOf([]int{})
		result, ok := convertArrayToSlice(cv, targetType)
		if !ok {
			t.Fatal("convertArrayToSlice failed")
		}

		got := result.([]int)
		want := []int{5, 10}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("uint slice", func(t *testing.T) {
		cv := ConfigValue{
			typ: TypeArray,
			array: []ConfigValue{
				{typ: TypeU64, u64: 100},
				{typ: TypeU64, u64: 200},
			},
		}

		targetType := reflect.TypeOf([]uint{})
		result, ok := convertArrayToSlice(cv, targetType)
		if !ok {
			t.Fatal("convertArrayToSlice failed")
		}

		got := result.([]uint)
		want := []uint{100, 200}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("string slice", func(t *testing.T) {
		cv := ConfigValue{
			typ: TypeArray,
			array: []ConfigValue{
				{typ: TypeString, str: "hello"},
				{typ: TypeString, str: "world"},
			},
		}

		targetType := reflect.TypeOf([]string{})
		result, ok := convertArrayToSlice(cv, targetType)
		if !ok {
			t.Fatal("convertArrayToSlice failed")
		}

		got := result.([]string)
		want := []string{"hello", "world"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("bool slice", func(t *testing.T) {
		cv := ConfigValue{
			typ: TypeArray,
			array: []ConfigValue{
				{typ: TypeBool, boolVal: true},
				{typ: TypeBool, boolVal: false},
				{typ: TypeBool, boolVal: true},
			},
		}

		targetType := reflect.TypeOf([]bool{})
		result, ok := convertArrayToSlice(cv, targetType)
		if !ok {
			t.Fatal("convertArrayToSlice failed")
		}

		got := result.([]bool)
		want := []bool{true, false, true}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("float64 slice", func(t *testing.T) {
		cv := ConfigValue{
			typ: TypeArray,
			array: []ConfigValue{
				{typ: TypeF64, f64: 1.1},
				{typ: TypeF64, f64: 2.2},
			},
		}

		targetType := reflect.TypeOf([]float64{})
		result, ok := convertArrayToSlice(cv, targetType)
		if !ok {
			t.Fatal("convertArrayToSlice failed")
		}

		got := result.([]float64)
		want := []float64{1.1, 2.2}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("empty array", func(t *testing.T) {
		cv := ConfigValue{
			typ:   TypeArray,
			array: []ConfigValue{},
		}

		targetType := reflect.TypeOf([]int64{})
		result, ok := convertArrayToSlice(cv, targetType)
		if !ok {
			t.Fatal("convertArrayToSlice failed")
		}

		got := result.([]int64)
		if len(got) != 0 {
			t.Errorf("len = %d, want 0", len(got))
		}
	})

	t.Run("not an array", func(t *testing.T) {
		cv := ConfigValue{typ: TypeU64, u64: 42}
		targetType := reflect.TypeOf([]int64{})
		_, ok := convertArrayToSlice(cv, targetType)
		if ok {
			t.Error("expected conversion to fail for non-array")
		}
	})

	t.Run("type mismatch in elements", func(t *testing.T) {
		cv := ConfigValue{
			typ: TypeArray,
			array: []ConfigValue{
				{typ: TypeString, str: "not a number"},
			},
		}

		targetType := reflect.TypeOf([]int64{})
		_, ok := convertArrayToSlice(cv, targetType)
		if ok {
			t.Error("expected conversion to fail for mismatched element types")
		}
	})

	t.Run("unsupported element type", func(t *testing.T) {
		cv := ConfigValue{
			typ: TypeArray,
			array: []ConfigValue{
				{typ: TypeU64, u64: 1},
			},
		}

		type customStruct struct{}
		targetType := reflect.TypeOf([]customStruct{})
		_, ok := convertArrayToSlice(cv, targetType)
		if ok {
			t.Error("expected conversion to fail for unsupported element type")
		}
	})

	t.Run("mixed valid types in array", func(t *testing.T) {
		// i64 can convert to uint64 if positive
		cv := ConfigValue{
			typ: TypeArray,
			array: []ConfigValue{
				{typ: TypeU64, u64: 10},
				{typ: TypeI64, i64: 20}, // positive i64
			},
		}

		targetType := reflect.TypeOf([]uint64{})
		result, ok := convertArrayToSlice(cv, targetType)
		if !ok {
			t.Fatal("convertArrayToSlice failed")
		}

		got := result.([]uint64)
		want := []uint64{10, 20}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("negative i64 to uint64 should fail", func(t *testing.T) {
		cv := ConfigValue{
			typ: TypeArray,
			array: []ConfigValue{
				{typ: TypeI64, i64: -1},
			},
		}

		targetType := reflect.TypeOf([]uint64{})
		_, ok := convertArrayToSlice(cv, targetType)
		if ok {
			t.Error("expected conversion to fail for negative i64 to uint64")
		}
	})
}
