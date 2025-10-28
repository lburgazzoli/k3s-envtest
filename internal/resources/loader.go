package resources

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lburgazzoli/k3s-envtest/internal/resources/filter"
	"github.com/lburgazzoli/k3s-envtest/internal/testutil"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// loadFromFile loads Kubernetes manifests from a single YAML file and applies the optional filter.
// Returns all objects if filter is nil.
func loadFromFile(
	filePath string,
	objectFilter filter.ObjectFilter,
) ([]unstructured.Unstructured, error) {
	//nolint:gosec // File path comes from trusted source
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	manifests, err := Decode(data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode YAML from %s: %w", filePath, err)
	}

	if objectFilter == nil {
		return manifests, nil
	}

	result := make([]unstructured.Unstructured, 0, len(manifests))
	for i := range manifests {
		if objectFilter(&manifests[i]) {
			result = append(result, manifests[i])
		}
	}

	return result, nil
}

// loadFromDirectory loads Kubernetes manifests from all YAML files in a directory (flat, non-recursive).
// Only processes files with .yaml or .yml extensions. Applies the optional filter.
// Returns all objects if filter is nil.
func loadFromDirectory(
	dir string,
	objectFilter filter.ObjectFilter,
) ([]unstructured.Unstructured, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var result []unstructured.Unstructured
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		fileName := entry.Name()
		ext := strings.ToLower(filepath.Ext(fileName))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		filePath := filepath.Join(dir, fileName)
		manifests, err := loadFromFile(filePath, objectFilter)
		if err != nil {
			return nil, err
		}
		result = append(result, manifests...)
	}

	return result, nil
}

// loadFromPath loads Kubernetes manifests from a file or directory.
// If the path is a directory, loads from all YAML files in it (flat, non-recursive).
// If the path is a file, loads from that file.
// Applies the optional filter. Returns all objects if filter is nil.
func loadFromPath(
	path string,
	objectFilter filter.ObjectFilter,
) ([]unstructured.Unstructured, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("manifest path does not exist: %s", path)
		}
		return nil, fmt.Errorf("failed to access manifest path %s: %w", path, err)
	}

	if info.IsDir() {
		return loadFromDirectory(path, objectFilter)
	}

	return loadFromFile(path, objectFilter)
}

// LoadFromPaths loads Kubernetes manifests from multiple paths (files or directories).
// Relative paths are resolved relative to the project root.
// Supports glob patterns in paths.
// Applies the optional filter. Returns all objects if filter is nil.
func LoadFromPaths(
	paths []string,
	objectFilter filter.ObjectFilter,
) ([]unstructured.Unstructured, error) {
	var result []unstructured.Unstructured

	for _, path := range paths {
		resolvedPath := path
		if !filepath.IsAbs(path) {
			projectRoot, err := testutil.FindProjectRoot()
			if err != nil {
				return nil, fmt.Errorf("failed to find project root for relative path %s: %w", path, err)
			}
			resolvedPath = filepath.Join(projectRoot, path)
		}

		// Check if path contains glob patterns
		if strings.ContainsAny(resolvedPath, "*?[]") {
			matches, err := filepath.Glob(resolvedPath)
			if err != nil {
				return nil, fmt.Errorf("failed to expand glob pattern %s: %w", resolvedPath, err)
			}

			for _, match := range matches {
				manifests, err := loadFromPath(match, objectFilter)
				if err != nil {
					return nil, err
				}
				result = append(result, manifests...)
			}
		} else {
			manifests, err := loadFromPath(resolvedPath, objectFilter)
			if err != nil {
				return nil, err
			}
			result = append(result, manifests...)
		}
	}

	return result, nil
}

// UnstructuredFromObjects converts client.Objects to unstructured.Unstructured objects.
// Ensures GVK is set on all objects before filtering (required for ByType filter).
// Returns all objects if filter is nil.
func UnstructuredFromObjects(
	scheme *runtime.Scheme,
	objects []client.Object,
	objectFilter filter.ObjectFilter,
) ([]unstructured.Unstructured, error) {
	result := make([]unstructured.Unstructured, 0, len(objects))

	for _, obj := range objects {
		// Ensure GVK is set before filtering (required for ByType filter to work)
		if err := EnsureGroupVersionKind(scheme, obj); err != nil {
			return nil, fmt.Errorf("failed to ensure GVK for object %T: %w", obj, err)
		}

		// Apply filter after GVK is set
		if objectFilter != nil && !objectFilter(obj) {
			continue
		}

		u, err := ToUnstructured(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to convert object to unstructured: %w", err)
		}

		result = append(result, *u.DeepCopy())
	}

	return result, nil
}
