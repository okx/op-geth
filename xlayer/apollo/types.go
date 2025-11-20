package apollo

import (
	"fmt"
	"math"
	"reflect"

	"github.com/ethereum/go-ethereum/log"
)

const (
	TypeU64 ConfigValueType = iota
	TypeU32
	TypeI64
	TypeI32
	TypeBool
	TypeString
	TypeF64
	TypeArray
)

type ConfigValueType uint8

type ConfigValue struct {
	// When adding a new type, update:
	// 1. ConfigValueType const (add enum)
	// 2. ConfigValue struct (add field)
	// 3. AsXXX() method (add getter)
	// 4. String() method (add case)
	// 5. common.go: GetConfigValueFromType() (add parser case)
	// 6. tryFromConfigValue() (add type switch case)
	// 7. utils.go: convertArrayToSlice() (if array support needed)

	typ     ConfigValueType
	u64     uint64
	i64     int64
	u32     uint32
	i32     int32
	str     string
	f64     float64
	boolVal bool
	array   []ConfigValue
}

func (cv ConfigValue) String() string {
	switch cv.typ {
	case TypeU64:
		return fmt.Sprintf("u64(%d)", cv.u64)
	case TypeI64:
		return fmt.Sprintf("i64(%d)", cv.i64)
	case TypeI32:
		return fmt.Sprintf("i32(%d)", cv.i32)
	case TypeU32:
		return fmt.Sprintf("u32(%d)", cv.u32)
	case TypeBool:
		return fmt.Sprintf("bool(%t)", cv.boolVal)
	case TypeString:
		return fmt.Sprintf("string(%q)", cv.str)
	case TypeF64:
		return fmt.Sprintf("f64(%f)", cv.f64)
	case TypeArray:
		return fmt.Sprintf("array%v", cv.array)
	default:
		return "unknown"
	}
}

func (cv *ConfigValue) AsString() (string, bool) {
	if cv.typ == TypeString {
		return cv.str, true
	}
	return "", false
}

func (cv *ConfigValue) AsUint64() (uint64, bool) {
	switch cv.typ {
	case TypeU64:
		return cv.u64, true
	case TypeI64:
		if cv.i64 >= 0 {
			return uint64(cv.i64), true
		}
	case TypeU32:
		return uint64(cv.u32), true

	case TypeI32:
		if cv.i32 >= 0 {
			return uint64(cv.i32), true
		}
	}
	return 0, false
}

func (cv *ConfigValue) AsInt64() (int64, bool) {
	switch cv.typ {
	case TypeI64:
		return cv.i64, true
	case TypeU64:
		if cv.u64 <= math.MaxInt64 {
			return int64(cv.u64), true
		}
	case TypeU32:
		return int64(cv.u32), true
	case TypeI32:
		return int64(cv.i32), true
	}
	return 0, false
}

func (cv *ConfigValue) AsInt32() (int32, bool) {
	switch cv.typ {
	case TypeI32:
		return cv.i32, true
	case TypeU32:
		if cv.u32 <= math.MaxInt32 {
			return int32(cv.u32), true
		}
	case TypeU64:
		if cv.u64 <= math.MaxInt32 {
			return int32(cv.u64), true
		}
	case TypeI64:
		if cv.i64 >= math.MinInt32 && cv.i64 <= math.MaxInt32 {
			return int32(cv.i64), true
		}
	}
	return 0, false
}

func (cv *ConfigValue) AsUint32() (uint32, bool) {
	switch cv.typ {
	case TypeU32:
		return cv.u32, true
	case TypeI32:
		if cv.i32 >= 0 {
			return uint32(cv.i32), true
		}
	case TypeU64:
		if cv.u64 <= math.MaxUint32 {
			return uint32(cv.u64), true
		}
	case TypeI64:
		if cv.i64 >= 0 && cv.i64 <= math.MaxUint32 {
			return uint32(cv.i64), true
		}
	}
	return 0, false
}

func (cv *ConfigValue) AsBool() (bool, bool) {
	if cv.typ == TypeBool {
		return cv.boolVal, true
	}
	return false, false
}

func (cv *ConfigValue) AsFloat64() (float64, bool) {
	switch cv.typ {
	case TypeF64:
		return cv.f64, true
	case TypeU64:
		return float64(cv.u64), true
	case TypeI64:
		return float64(cv.i64), true
	}
	return 0, false
}

func (cv ConfigValue) AsArray() ([]ConfigValue, bool) {
	if cv.typ == TypeArray {
		values := make([]ConfigValue, len(cv.array))
		for i, v := range cv.array {
			values[i] = v
		}
		return values, true
	}
	return nil, false
}

func tryFromConfigValue[T any](configVal ConfigValue) (T, bool) {
	var defaultValue T
	defaultType := reflect.TypeOf(defaultValue)

	if defaultType.Kind() == reflect.Slice {
		result, success := convertArrayToSlice(configVal, defaultType)
		if success {
			return result.(T), true
		}
		return defaultValue, false
	}

	var result any
	var success bool

	switch any(defaultValue).(type) {
	case uint64:
		result, success = configVal.AsUint64()
	case int64:
		result, success = configVal.AsInt64()
	case int32:
		result, success = configVal.AsInt32()
	case uint32:
		result, success = configVal.AsUint32()
	case int:
		val, ok := configVal.AsInt64()
		result, success = int(val), ok
	case string:
		result, success = configVal.AsString()
	case bool:
		result, success = configVal.AsBool()
	case float64:
		result, success = configVal.AsFloat64()

	default:
		log.Warn("[Apollo] Using default (unsupported type)")
		return defaultValue, false
	}

	if !success {
		log.Warn("[Apollo] Using default (type mismatch)")
		return defaultValue, false
	}
	return result.(T), success
}
