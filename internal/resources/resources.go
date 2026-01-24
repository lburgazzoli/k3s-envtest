package resources

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

func ToUnstructured(obj any) (*unstructured.Unstructured, error) {
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, fmt.Errorf("unable to convert object %T to unstructured: %w", obj, err)
	}

	u := unstructured.Unstructured{
		Object: data,
	}

	return &u, nil
}

// YAMLToUnstructured converts a YAML string to an unstructured object.
// This is useful for testing purposes.
func YAMLToUnstructured(yamlStr string) (*unstructured.Unstructured, error) {
	var data map[string]any
	if err := yaml.Unmarshal([]byte(yamlStr), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml: %w", err)
	}

	return &unstructured.Unstructured{Object: data}, nil
}

func GetGroupVersionKindForObject(
	s *runtime.Scheme,
	obj runtime.Object,
) (schema.GroupVersionKind, error) {
	if obj == nil {
		return schema.GroupVersionKind{}, errors.New("nil object")
	}

	if obj.GetObjectKind().GroupVersionKind().Version != "" && obj.GetObjectKind().GroupVersionKind().Kind != "" {
		return obj.GetObjectKind().GroupVersionKind(), nil
	}

	gvk, err := apiutil.GVKForObject(obj, s)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("failed to get GVK: %w", err)
	}

	return gvk, nil
}

func EnsureGroupVersionKind(
	s *runtime.Scheme,
	obj client.Object,
) error {
	gvk, err := GetGroupVersionKindForObject(s, obj)
	if err != nil {
		return err
	}

	obj.GetObjectKind().SetGroupVersionKind(gvk)

	return nil
}

// Convert converts an unstructured object to a typed object and ensures GVK is set.
func Convert[T client.Object](
	scheme *runtime.Scheme,
	src *unstructured.Unstructured,
	dst T,
) error {
	if err := scheme.Convert(src, dst, nil); err != nil {
		return fmt.Errorf("failed to convert object: %w", err)
	}
	if err := EnsureGroupVersionKind(scheme, dst); err != nil {
		return fmt.Errorf("failed to ensure GVK for object: %w", err)
	}
	return nil
}

func FormatObjectReference(u client.Object) string {
	gvk := u.GetObjectKind().GroupVersionKind().String()
	name := u.GetName()
	ns := u.GetNamespace()
	if ns != "" {
		return gvk + " " + ns + "/" + name
	}
	return gvk + " " + name
}

func Decode(content []byte) ([]unstructured.Unstructured, error) {
	results := make([]unstructured.Unstructured, 0)

	r := bytes.NewReader(content)
	yd := yaml.NewDecoder(r)

	for {
		var out map[string]any

		err := yd.Decode(&out)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("unable to decode resource: %w", err)
		}

		if len(out) == 0 {
			continue
		}

		kind, ok := out["kind"]
		if !ok || kind == nil || kind == "" {
			continue
		}

		obj, err := ToUnstructured(&out)
		if err != nil {
			return nil, fmt.Errorf("unable to convert to unstructured: %w", err)
		}

		results = append(results, *obj)
	}

	return results, nil
}

// AllConvertibleTypes returns a set of all GroupKind types in the scheme
// that support conversion between versions.
func AllConvertibleTypes(scheme *runtime.Scheme) (sets.Set[schema.GroupKind], error) {
	convertibles := sets.New[schema.GroupKind]()
	for gvk := range scheme.AllKnownTypes() {
		obj, err := scheme.New(gvk)
		if err != nil {
			return nil, fmt.Errorf("failed to create object for %s: %w", gvk, err)
		}
		if ok, err := conversion.IsConvertible(scheme, obj); ok && err == nil {
			convertibles.Insert(gvk.GroupKind())
		}
	}
	return convertibles, nil
}
