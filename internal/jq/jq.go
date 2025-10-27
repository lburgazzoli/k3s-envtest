package jq

import (
	"fmt"

	"github.com/itchyny/gojq"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Transform applies a JQ transformation to an unstructured object, mutating it in place.
func Transform(
	obj *unstructured.Unstructured,
	expression string,
	args ...interface{},
) error {
	query, err := gojq.Parse(fmt.Sprintf(expression, args...))
	if err != nil {
		return fmt.Errorf("failed to parse jq expression: %w", err)
	}

	result, ok := query.Run(obj.Object).Next()
	if !ok || result == nil {
		return nil
	}

	if err, ok := result.(error); ok {
		return fmt.Errorf("jq execution error: %w", err)
	}

	transformed, ok := result.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected map[string]interface{}, got %T", result)
	}

	obj.SetUnstructuredContent(transformed)

	return nil
}

// Query executes a JQ query and returns a typed result.
// Use any as the type parameter for untyped queries.
//
// Example:
//
//	name, err := jq.Query[string](obj, `.metadata.name`)
//	enabled, err := jq.Query[bool](obj, `.spec.enabled`)
//	result, err := jq.Query[any](obj, `.metadata`)  // untyped
func Query[T any](
	obj *unstructured.Unstructured,
	expression string,
	args ...any,
) (T, error) {
	var zero T
	query, err := gojq.Parse(fmt.Sprintf(expression, args...))
	if err != nil {
		return zero, fmt.Errorf("failed to parse jq expression: %w", err)
	}

	result, ok := query.Run(obj.Object).Next()
	if !ok {
		return zero, nil
	}

	if err, ok := result.(error); ok {
		return zero, fmt.Errorf("jq execution error: %w", err)
	}

	// Handle nil result
	if result == nil {
		return zero, nil
	}

	typed, ok := result.(T)
	if !ok {
		return zero, fmt.Errorf("expected type %T, got %T", zero, result)
	}

	return typed, nil
}

// QuerySlice executes a JQ query and returns a typed slice.
// Automatically handles the []interface{} conversion and type assertions for each element.
//
// Example:
//
//	urls, err := jq.QuerySlice[string](obj, `[.webhooks[].clientConfig.url]`)
//	ports, err := jq.QuerySlice[float64](obj, `[.spec.ports[].port]`)
func QuerySlice[T any](
	obj *unstructured.Unstructured,
	expression string,
	args ...any,
) ([]T, error) {
	result, err := Query[any](obj, expression, args...)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	arr, ok := result.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected array result, got %T", result)
	}

	typed := make([]T, 0, len(arr))
	for i, item := range arr {
		t, ok := item.(T)
		if !ok {
			var zero T
			return nil, fmt.Errorf("expected type %T at index %d, got %T", zero, i, item)
		}
		typed = append(typed, t)
	}

	return typed, nil
}

// QueryMap executes a JQ query and returns a typed map.
// Useful for queries that return objects/maps with known key/value types.
//
// Example:
//
//	labels, err := jq.QueryMap[string, string](obj, `.metadata.labels`)
func QueryMap[K comparable, V any](
	obj *unstructured.Unstructured,
	expression string,
	args ...any,
) (map[K]V, error) {
	result, err := Query[any](obj, expression, args...)
	if err != nil {
		return nil, err
	}

	if result == nil {
		// No result from query is not an error
		//nolint:nilnil
		return nil, nil
	}

	rawMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected map result, got %T", result)
	}

	typed := make(map[K]V, len(rawMap))
	for key, value := range rawMap {
		k, ok := any(key).(K)
		if !ok {
			var zeroK K
			return nil, fmt.Errorf("expected key type %T, got %T for key %v", zeroK, key, key)
		}

		v, ok := value.(V)
		if !ok {
			var zeroV V
			return nil, fmt.Errorf("expected value type %T, got %T for key %v", zeroV, value, key)
		}

		typed[k] = v
	}

	return typed, nil
}
