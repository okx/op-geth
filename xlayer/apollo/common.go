package apollo

import (
	"fmt"
	"reflect"
)

// This function is mainly used to convert the config value to the ConfigValue type during updating of the cache.
func (a *ApolloService) GetConfigValueFromType(value interface{}) (ConfigValue, error) {
	var err error

	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Slice {
		length := rv.Len()
		s := make([]ConfigValue, length)

		for i := 0; i < length; i++ {
			elem := rv.Index(i).Interface()
			s[i], err = a.GetConfigValueFromType(elem)
			if err != nil {
				return ConfigValue{}, fmt.Errorf("array element %d: %w", i, err)
			}
		}
		return ConfigValue{
			typ:   TypeArray,
			array: s,
		}, nil
	}

	switch value.(type) {
	case uint64:
		return ConfigValue{
			typ: TypeU64,
			u64: value.(uint64),
		}, nil
	case int64:
		return ConfigValue{
			typ: TypeI64,
			i64: value.(int64),
		}, nil
	case uint32:
		return ConfigValue{
			typ: TypeU32,
			u32: value.(uint32),
		}, nil
	case int32:
		return ConfigValue{
			typ: TypeI32,
			i32: value.(int32),
		}, nil
	case int:
		return ConfigValue{
			typ: TypeI64,
			i64: int64(value.(int)),
		}, nil
	case bool:
		return ConfigValue{
			typ:     TypeBool,
			boolVal: value.(bool),
		}, nil
	case string:
		return ConfigValue{
			typ: TypeString,
			str: value.(string),
		}, nil
	case float64:
		return ConfigValue{
			typ: TypeF64,
			f64: value.(float64),
		}, nil
	default:
		return ConfigValue{
			typ: TypeString,
			str: fmt.Sprintf("%v", value),
		}, nil
	}
}
