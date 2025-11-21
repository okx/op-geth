package apollo

import (
	"math"
	"testing"
)

// TestConfigValue_AsString tests string conversion
func TestConfigValue_AsString(t *testing.T) {
	tests := []struct {
		name     string
		cv       ConfigValue
		want     string
		wantBool bool
	}{
		{
			name:     "valid string",
			cv:       ConfigValue{typ: TypeString, str: "hello"},
			want:     "hello",
			wantBool: true,
		},
		{
			name:     "empty string",
			cv:       ConfigValue{typ: TypeString, str: ""},
			want:     "",
			wantBool: true,
		},
		{
			name:     "wrong type - uint64",
			cv:       ConfigValue{typ: TypeU64, u64: 42},
			want:     "",
			wantBool: false,
		},
		{
			name:     "wrong type - bool",
			cv:       ConfigValue{typ: TypeBool, boolVal: true},
			want:     "",
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.cv.AsString()
			if ok != tt.wantBool {
				t.Errorf("AsString() ok = %v, want %v", ok, tt.wantBool)
			}
			if got != tt.want {
				t.Errorf("AsString() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestConfigValue_AsUint64 tests uint64 conversion with cross-type conversions
func TestConfigValue_AsUint64(t *testing.T) {
	tests := []struct {
		name     string
		cv       ConfigValue
		want     uint64
		wantBool bool
	}{
		{
			name:     "direct uint64",
			cv:       ConfigValue{typ: TypeU64, u64: 12345},
			want:     12345,
			wantBool: true,
		},
		{
			name:     "positive int64 to uint64",
			cv:       ConfigValue{typ: TypeI64, i64: 42},
			want:     42,
			wantBool: true,
		},
		{
			name:     "negative int64 to uint64",
			cv:       ConfigValue{typ: TypeI64, i64: -1},
			want:     0,
			wantBool: false,
		},
		{
			name:     "wrong type - string",
			cv:       ConfigValue{typ: TypeString, str: "123"},
			want:     0,
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.cv.AsUint64()
			if ok != tt.wantBool {
				t.Errorf("AsUint64() ok = %v, want %v", ok, tt.wantBool)
			}
			if got != tt.want {
				t.Errorf("AsUint64() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestConfigValue_AsInt64 tests int64 conversion with overflow checks
func TestConfigValue_AsInt64(t *testing.T) {
	tests := []struct {
		name     string
		cv       ConfigValue
		want     int64
		wantBool bool
	}{
		{
			name:     "direct int64",
			cv:       ConfigValue{typ: TypeI64, i64: -42},
			want:     -42,
			wantBool: true,
		},
		{
			name:     "uint64 within range",
			cv:       ConfigValue{typ: TypeU64, u64: 1000},
			want:     1000,
			wantBool: true,
		},
		{
			name:     "uint64 at max int64",
			cv:       ConfigValue{typ: TypeU64, u64: math.MaxInt64},
			want:     math.MaxInt64,
			wantBool: true,
		},
		{
			name:     "uint64 overflow",
			cv:       ConfigValue{typ: TypeU64, u64: math.MaxUint64},
			want:     0,
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.cv.AsInt64()
			if ok != tt.wantBool {
				t.Errorf("AsInt64() ok = %v, want %v", ok, tt.wantBool)
			}
			if got != tt.want {
				t.Errorf("AsInt64() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestConfigValue_AsBool tests boolean conversion
func TestConfigValue_AsBool(t *testing.T) {
	tests := []struct {
		name     string
		cv       ConfigValue
		want     bool
		wantBool bool
	}{
		{
			name:     "true value",
			cv:       ConfigValue{typ: TypeBool, boolVal: true},
			want:     true,
			wantBool: true,
		},
		{
			name:     "false value",
			cv:       ConfigValue{typ: TypeBool, boolVal: false},
			want:     false,
			wantBool: true,
		},
		{
			name:     "wrong type",
			cv:       ConfigValue{typ: TypeU64, u64: 1},
			want:     false,
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.cv.AsBool()
			if ok != tt.wantBool {
				t.Errorf("AsBool() ok = %v, want %v", ok, tt.wantBool)
			}
			if got != tt.want {
				t.Errorf("AsBool() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestConfigValue_AsFloat64 tests float conversion with type coercion
func TestConfigValue_AsFloat64(t *testing.T) {
	tests := []struct {
		name     string
		cv       ConfigValue
		want     float64
		wantBool bool
	}{
		{
			name:     "direct float64",
			cv:       ConfigValue{typ: TypeF64, f64: 3.14},
			want:     3.14,
			wantBool: true,
		},
		{
			name:     "uint64 to float64",
			cv:       ConfigValue{typ: TypeU64, u64: 100},
			want:     100.0,
			wantBool: true,
		},
		{
			name:     "int64 to float64",
			cv:       ConfigValue{typ: TypeI64, i64: -50},
			want:     -50.0,
			wantBool: true,
		},
		{
			name:     "wrong type - string",
			cv:       ConfigValue{typ: TypeString, str: "3.14"},
			want:     0,
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.cv.AsFloat64()
			if ok != tt.wantBool {
				t.Errorf("AsFloat64() ok = %v, want %v", ok, tt.wantBool)
			}
			if got != tt.want {
				t.Errorf("AsFloat64() = %v, want %v", got, tt.want)
			}
		})
	}
}
func TestConfigValue_AsArray(t *testing.T) {
	tests := []struct {
		name     string
		cv       ConfigValue
		want     []ConfigValue
		wantBool bool
	}{
		{
			name: "valid array",
			cv: ConfigValue{
				typ: TypeArray,
				array: []ConfigValue{
					{typ: TypeU64, u64: 1},
					{typ: TypeU64, u64: 2},
					{typ: TypeU64, u64: 3},
				},
			},
			want: []ConfigValue{
				{typ: TypeU64, u64: 1},
				{typ: TypeU64, u64: 2},
				{typ: TypeU64, u64: 3},
			},
			wantBool: true,
		},
		{
			name: "empty array",
			cv: ConfigValue{
				typ:   TypeArray,
				array: []ConfigValue{},
			},
			want:     []ConfigValue{},
			wantBool: true,
		},
		{
			name:     "wrong type",
			cv:       ConfigValue{typ: TypeU64, u64: 42},
			want:     nil,
			wantBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.cv.AsArray()
			if ok != tt.wantBool {
				t.Errorf("AsArray() ok = %v, want %v", ok, tt.wantBool)
			}
			if !ok {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("AsArray() len = %v, want %v", len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i].typ != tt.want[i].typ {
					t.Errorf("AsArray()[%d].typ = %v, want %v", i, got[i].typ, tt.want[i].typ)
				}
			}
		})
	}
}

// TestConfigValue_String tests string representation (for logging only)
func TestConfigValue_String(t *testing.T) {
	tests := []struct {
		name string
		cv   ConfigValue
		want string
	}{
		{
			name: "uint64",
			cv:   ConfigValue{typ: TypeU64, u64: 42},
			want: "u64(42)",
		},
		{
			name: "int64",
			cv:   ConfigValue{typ: TypeI64, i64: -123},
			want: "i64(-123)",
		},
		{
			name: "bool true",
			cv:   ConfigValue{typ: TypeBool, boolVal: true},
			want: "bool(true)",
		},
		{
			name: "bool false",
			cv:   ConfigValue{typ: TypeBool, boolVal: false},
			want: "bool(false)",
		},
		{
			name: "string",
			cv:   ConfigValue{typ: TypeString, str: "hello"},
			want: `string("hello")`,
		},
		{
			name: "float64",
			cv:   ConfigValue{typ: TypeF64, f64: 3.14},
			want: "f64(3.140000)",
		},
		{
			name: "array",
			cv: ConfigValue{
				typ: TypeArray,
				array: []ConfigValue{
					{typ: TypeU64, u64: 1},
				},
			},
			want: "array[u64(1)]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cv.String()
			if got != tt.want {
				t.Errorf("String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestTryFromConfigValue tests the generic conversion function
func TestTryFromConfigValue(t *testing.T) {
	t.Run("uint64 conversion", func(t *testing.T) {
		cv := ConfigValue{typ: TypeU64, u64: 12345}
		got, ok := tryFromConfigValue[uint64](cv)
		if !ok {
			t.Fatal("tryFromConfigValue failed")
		}
		if got != 12345 {
			t.Errorf("got %v, want 12345", got)
		}
	})

	t.Run("int64 conversion", func(t *testing.T) {
		cv := ConfigValue{typ: TypeI64, i64: -999}
		got, ok := tryFromConfigValue[int64](cv)
		if !ok {
			t.Fatal("tryFromConfigValue failed")
		}
		if got != -999 {
			t.Errorf("got %v, want -999", got)
		}
	})

	t.Run("int conversion", func(t *testing.T) {
		cv := ConfigValue{typ: TypeI64, i64: 42}
		got, ok := tryFromConfigValue[int](cv)
		if !ok {
			t.Fatal("tryFromConfigValue failed")
		}
		if got != 42 {
			t.Errorf("got %v, want 42", got)
		}
	})

	t.Run("string conversion", func(t *testing.T) {
		cv := ConfigValue{typ: TypeString, str: "test"}
		got, ok := tryFromConfigValue[string](cv)
		if !ok {
			t.Fatal("tryFromConfigValue failed")
		}
		if got != "test" {
			t.Errorf("got %v, want test", got)
		}
	})

	t.Run("bool conversion", func(t *testing.T) {
		cv := ConfigValue{typ: TypeBool, boolVal: true}
		got, ok := tryFromConfigValue[bool](cv)
		if !ok {
			t.Fatal("tryFromConfigValue failed")
		}
		if got != true {
			t.Errorf("got %v, want true", got)
		}
	})

	t.Run("float64 conversion", func(t *testing.T) {
		cv := ConfigValue{typ: TypeF64, f64: 3.14}
		got, ok := tryFromConfigValue[float64](cv)
		if !ok {
			t.Fatal("tryFromConfigValue failed")
		}
		if got != 3.14 {
			t.Errorf("got %v, want 3.14", got)
		}
	})

	t.Run("slice conversion", func(t *testing.T) {
		cv := ConfigValue{
			typ: TypeArray,
			array: []ConfigValue{
				{typ: TypeI64, i64: 1},
				{typ: TypeI64, i64: 2},
				{typ: TypeI64, i64: 3},
			},
		}
		got, ok := tryFromConfigValue[[]int64](cv)
		if !ok {
			t.Fatal("tryFromConfigValue failed")
		}
		want := []int64{1, 2, 3}
		if len(got) != len(want) {
			t.Fatalf("len = %v, want %v", len(got), len(want))
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("got[%d] = %v, want %v", i, got[i], want[i])
			}
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		cv := ConfigValue{typ: TypeString, str: "not a number"}
		_, ok := tryFromConfigValue[uint64](cv)
		if ok {
			t.Error("expected conversion to fail")
		}
	})

	t.Run("unsupported type", func(t *testing.T) {
		type customType struct{}
		cv := ConfigValue{typ: TypeString, str: "test"}
		_, ok := tryFromConfigValue[customType](cv)
		if ok {
			t.Error("expected conversion to fail for unsupported type")
		}
	})
}
