package apollo

import (
	"reflect"
	"strings"

	"github.com/ethereum/go-ethereum/log"
)

func getNamespacePrefix(namespace string) string {
	if strings.Contains(namespace, "-") {
		prefix := strings.Split(namespace, "-")[0]
		return prefix
	}
	return namespace
}

func convertArrayToSlice(configVal ConfigValue, targetType reflect.Type) (any, bool) {
	arr, ok := configVal.AsArray()
	if !ok {
		return nil, false
	}

	elemType := targetType.Elem()
	slice := reflect.MakeSlice(targetType, len(arr), len(arr))

	for i, cv := range arr {
		var elem any
		var success bool

		switch elemType.Kind() {
		case reflect.Int64:
			elem, success = cv.AsInt64()
		case reflect.Uint64:
			elem, success = cv.AsUint64()
		case reflect.Int32:
			elem, success = cv.AsInt32()
		case reflect.Uint32:
			elem, success = cv.AsUint32()
		case reflect.Int:
			val, ok := cv.AsInt64()
			elem, success = int(val), ok
		case reflect.Uint:
			val, ok := cv.AsUint64()
			elem, success = uint(val), ok
		case reflect.String:
			elem, success = cv.AsString()
		case reflect.Bool:
			elem, success = cv.AsBool()
		case reflect.Float64:
			elem, success = cv.AsFloat64()
		default:
			return nil, false
		}

		if !success {
			log.Warn("[Apollo] Array element conversion failed", "index", i, "type", elemType)
			return nil, false
		}

		slice.Index(i).Set(reflect.ValueOf(elem))
	}

	return slice.Interface(), true
}
