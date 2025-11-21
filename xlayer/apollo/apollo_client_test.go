package apollo

import (
	"testing"

	"gopkg.in/yaml.v2"
)

// TestMakeCacheKey tests cache key generation
func TestMakeCacheKey(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		key       string
		want      string
	}{
		{
			name:      "simple namespace",
			namespace: "jsonrpc",
			key:       "max_batch_size",
			want:      "jsonrpc:max_batch_size",
		},
		{
			name:      "namespace with suffix",
			namespace: "opgeth_l2gaspricer-config",
			key:       "gas_price",
			want:      "opgeth_l2gaspricer:gas_price",
		},
		{
			name:      "namespace with multiple parts",
			namespace: "opnode_sequencer-config",
			key:       "block_time",
			want:      "opnode_sequencer:block_time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := makeCacheKey(tt.namespace, tt.key)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("makeCacheKey() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestApolloService_updateCacheFromConfig tests cache update from config string
func TestApolloService_updateCacheFromConfig(t *testing.T) {
	t.Run("valid yaml config", func(t *testing.T) {
		svc := &ApolloService{
			cache: make(map[string]ConfigValue),
		}

		yamlConfig := `
max_batch_size: 100
timeout: 30
enabled: true
name: "test"
rate: 3.14
`
		err := svc.updateCacheFromConfig("jsonrpc", yamlConfig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check cache was populated
		if len(svc.cache) == 0 {
			t.Error("cache is empty")
		}

		// Verify specific values
		key := "jsonrpc:max_batch_size"
		cv, ok := svc.cache[key]
		if !ok {
			t.Errorf("key %s not found in cache", key)
		}
		val, ok := cv.AsInt64()
		if !ok || val != 100 {
			t.Errorf("max_batch_size = %v, want 100", val)
		}
	})

	t.Run("yaml with array", func(t *testing.T) {
		svc := &ApolloService{
			cache: make(map[string]ConfigValue),
		}

		yamlConfig := `allowed_ips: [1, 2, 3]`

		err := svc.updateCacheFromConfig("jsonrpc", yamlConfig)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		key := "jsonrpc:allowed_ips"
		cv, ok := svc.cache[key]
		if !ok {
			t.Errorf("key %s not found in cache", key)
		}

		arr, ok := cv.AsArray()
		if !ok {
			t.Fatal("expected array type")
		}
		if len(arr) != 3 {
			t.Errorf("array length = %d, want 3", len(arr))
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		svc := &ApolloService{
			cache: make(map[string]ConfigValue),
		}

		invalidYaml := `
		this is not
		valid: yaml: syntax
		`
		err := svc.updateCacheFromConfig("jsonrpc", invalidYaml)
		if err == nil {
			t.Error("expected error for invalid yaml")
		}
	})

	t.Run("non-string value from apollo", func(t *testing.T) {
		svc := &ApolloService{
			cache: make(map[string]ConfigValue),
		}

		err := svc.updateCacheFromConfig("jsonrpc", 12345)
		if err == nil {
			t.Error("expected error for non-string value")
		}
	})

	t.Run("empty config", func(t *testing.T) {
		svc := &ApolloService{
			cache: make(map[string]ConfigValue),
		}

		err := svc.updateCacheFromConfig("jsonrpc", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Empty YAML should result in empty cache (or no changes)
	})

	t.Run("concurrent cache updates", func(t *testing.T) {
		svc := &ApolloService{
			cache: make(map[string]ConfigValue),
		}

		yamlConfig := `
key1: 100
key2: 200
`
		// Test thread safety with concurrent updates
		done := make(chan bool, 2)
		go func() {
			_ = svc.updateCacheFromConfig("ns1", yamlConfig)
			done <- true
		}()
		go func() {
			_ = svc.updateCacheFromConfig("ns2", yamlConfig)
			done <- true
		}()

		<-done
		<-done

		// Both namespaces should have their keys
		if _, ok := svc.cache["ns1:key1"]; !ok {
			t.Error("ns1:key1 not found")
		}
		if _, ok := svc.cache["ns2:key1"]; !ok {
			t.Error("ns2:key1 not found")
		}
	})
}

// TestApolloService_getCachedConfig tests cache retrieval
func TestApolloService_getCachedConfig(t *testing.T) {
	t.Run("existing key", func(t *testing.T) {
		svc := &ApolloService{
			cache: map[string]ConfigValue{
				"test:key": {typ: TypeU64, u64: 42},
			},
		}

		cv, ok := svc.getCachedConfig("test:key")
		if !ok {
			t.Error("expected to find key")
		}
		val, _ := cv.AsUint64()
		if val != 42 {
			t.Errorf("value = %v, want 42", val)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		svc := &ApolloService{
			cache: make(map[string]ConfigValue),
		}

		_, ok := svc.getCachedConfig("nonexistent")
		if ok {
			t.Error("expected key not to be found")
		}
	})

	t.Run("nil cache", func(t *testing.T) {
		svc := &ApolloService{
			cache: nil,
		}

		_, ok := svc.getCachedConfig("test:key")
		if ok {
			t.Error("expected false for nil cache")
		}
	})

	t.Run("concurrent reads", func(t *testing.T) {
		svc := &ApolloService{
			cache: map[string]ConfigValue{
				"test:key": {typ: TypeU64, u64: 42},
			},
		}

		// Test thread safety with concurrent reads
		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func() {
				_, ok := svc.getCachedConfig("test:key")
				if !ok {
					t.Error("expected to find key")
				}
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// TestApolloConfigOr tests the public API
func TestApolloConfigOr(t *testing.T) {
	// Save original instance and restore after test
	originalInstance := instance
	defer func() { instance = originalInstance }()

	t.Run("nil instance returns default", func(t *testing.T) {
		instance = nil
		val, ok := ApolloConfigOr("jsonrpc", "timeout", uint64(30))
		if ok {
			t.Error("expected ok=false for nil instance")
		}
		if val != 30 {
			t.Errorf("value = %v, want default 30", val)
		}
	})

	t.Run("missing key returns default", func(t *testing.T) {
		instance = &ApolloService{
			cache: make(map[string]ConfigValue),
		}

		val, ok := ApolloConfigOr("jsonrpc", "nonexistent", uint64(99))
		if ok {
			t.Error("expected ok=false for missing key")
		}
		if val != 99 {
			t.Errorf("value = %v, want default 99", val)
		}
	})

	t.Run("successful uint64 retrieval", func(t *testing.T) {
		instance = &ApolloService{
			cache: map[string]ConfigValue{
				"jsonrpc:timeout": {typ: TypeU64, u64: 60},
			},
		}

		val, ok := ApolloConfigOr("jsonrpc", "timeout", uint64(30))
		if !ok {
			t.Error("expected ok=true")
		}
		if val != 60 {
			t.Errorf("value = %v, want 60", val)
		}
	})

	t.Run("successful string retrieval", func(t *testing.T) {
		instance = &ApolloService{
			cache: map[string]ConfigValue{
				"jsonrpc:host": {typ: TypeString, str: "localhost"},
			},
		}

		val, ok := ApolloConfigOr("jsonrpc", "host", "default")
		if !ok {
			t.Error("expected ok=true")
		}
		if val != "localhost" {
			t.Errorf("value = %v, want localhost", val)
		}
	})

	t.Run("successful bool retrieval", func(t *testing.T) {
		instance = &ApolloService{
			cache: map[string]ConfigValue{
				"jsonrpc:enabled": {typ: TypeBool, boolVal: true},
			},
		}

		val, ok := ApolloConfigOr("jsonrpc", "enabled", false)
		if !ok {
			t.Error("expected ok=true")
		}
		if val != true {
			t.Errorf("value = %v, want true", val)
		}
	})

	t.Run("successful slice retrieval", func(t *testing.T) {
		instance = &ApolloService{
			cache: map[string]ConfigValue{
				"jsonrpc:ports": {
					typ: TypeArray,
					array: []ConfigValue{
						{typ: TypeI64, i64: 8080},
						{typ: TypeI64, i64: 8081},
					},
				},
			},
		}

		val, ok := ApolloConfigOr("jsonrpc", "ports", []int64{})
		if !ok {
			t.Error("expected ok=true")
		}
		want := []int64{8080, 8081}
		if len(val) != len(want) {
			t.Fatalf("len = %d, want %d", len(val), len(want))
		}
		for i := range val {
			if val[i] != want[i] {
				t.Errorf("val[%d] = %v, want %v", i, val[i], want[i])
			}
		}
	})

	t.Run("type mismatch returns default", func(t *testing.T) {
		instance = &ApolloService{
			cache: map[string]ConfigValue{
				"jsonrpc:timeout": {typ: TypeString, str: "not a number"},
			},
		}

		val, ok := ApolloConfigOr("jsonrpc", "timeout", uint64(30))
		if ok {
			t.Error("expected ok=false for type mismatch")
		}
		if val != 30 {
			t.Errorf("value = %v, want default 30", val)
		}
	})
}

// TestYAMLIntegration tests actual YAML unmarshaling behavior
func TestYAMLIntegration(t *testing.T) {
	t.Run("yaml number types", func(t *testing.T) {
		yamlStr := `
int_value: 42
float_value: 3.14
bool_value: true
string_value: hello
array_value: [1, 2, 3]
`
		var config map[string]interface{}
		err := yaml.Unmarshal([]byte(yamlStr), &config)
		if err != nil {
			t.Fatalf("yaml unmarshal error: %v", err)
		}

		svc := &ApolloService{}

		// YAML unmarshals numbers as int by default
		intCV, err := svc.GetConfigValueFromType(config["int_value"])
		if err != nil {
			t.Errorf("int_value error: %v", err)
		}
		if intCV.typ != TypeI64 {
			t.Errorf("int_value type = %v, want TypeI64", intCV.typ)
		}

		// YAML unmarshals floats as float64
		floatCV, err := svc.GetConfigValueFromType(config["float_value"])
		if err != nil {
			t.Errorf("float_value error: %v", err)
		}
		if floatCV.typ != TypeF64 {
			t.Errorf("float_value type = %v, want TypeF64", floatCV.typ)
		}

		// Bool
		boolCV, err := svc.GetConfigValueFromType(config["bool_value"])
		if err != nil {
			t.Errorf("bool_value error: %v", err)
		}
		if boolCV.typ != TypeBool {
			t.Errorf("bool_value type = %v, want TypeBool", boolCV.typ)
		}

		// String
		strCV, err := svc.GetConfigValueFromType(config["string_value"])
		if err != nil {
			t.Errorf("string_value error: %v", err)
		}
		if strCV.typ != TypeString {
			t.Errorf("string_value type = %v, want TypeString", strCV.typ)
		}

		// Array
		arrayCV, err := svc.GetConfigValueFromType(config["array_value"])
		if err != nil {
			t.Errorf("array_value error: %v", err)
		}
		if arrayCV.typ != TypeArray {
			t.Errorf("array_value type = %v, want TypeArray", arrayCV.typ)
		}
	})
}
