package resources_test

import (
	"testing"

	"github.com/lburgazzoli/k3s-envtest/internal/resources"
	"github.com/lburgazzoli/k3s-envtest/internal/testdata/v1alpha1"
	"github.com/lburgazzoli/k3s-envtest/internal/testdata/v1beta1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/gomega"
)

func TestFilterConvertibleCRDs_Success(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())
	err = v1beta1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	convertibleCRD := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "sampleresources.example.k3senv.io",
			},
			"spec": map[string]interface{}{
				"group": "example.k3senv.io",
				"names": map[string]interface{}{
					"kind":   "SampleResource",
					"plural": "sampleresources",
				},
			},
		},
	}

	nonConvertibleCRD := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "others.other.io",
			},
			"spec": map[string]interface{}{
				"group": "other.io",
				"names": map[string]interface{}{
					"kind":   "Other",
					"plural": "others",
				},
			},
		},
	}

	crds := []unstructured.Unstructured{*convertibleCRD, *nonConvertibleCRD}

	result, err := resources.FilterConvertibleCRDs(scheme, crds)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(HaveLen(1))
	g.Expect(result[0].GetName()).To(Equal("sampleresources.example.k3senv.io"))
}

func TestFilterConvertibleCRDs_EmptyList(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	result, err := resources.FilterConvertibleCRDs(scheme, []unstructured.Unstructured{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(BeEmpty())
}

func TestFilterConvertibleCRDs_NonConvertible(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	nonConvertibleCRD := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "others.other.io",
			},
			"spec": map[string]interface{}{
				"group": "other.io",
				"names": map[string]interface{}{
					"kind":   "Other",
					"plural": "others",
				},
			},
		},
	}

	crds := []unstructured.Unstructured{*nonConvertibleCRD}

	result, err := resources.FilterConvertibleCRDs(scheme, crds)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(BeEmpty())
}

func TestFilterConvertibleCRDs_MissingGroup(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	crdWithoutGroup := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "sampleresources.example.k3senv.io",
			},
			"spec": map[string]interface{}{
				"names": map[string]interface{}{
					"kind":   "SampleResource",
					"plural": "sampleresources",
				},
			},
		},
	}

	crds := []unstructured.Unstructured{*crdWithoutGroup}

	_, err = resources.FilterConvertibleCRDs(scheme, crds)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("missing required field: spec.group"))
}

func TestFilterConvertibleCRDs_MissingKind(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	crdWithoutKind := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "sampleresources.example.k3senv.io",
			},
			"spec": map[string]interface{}{
				"group": "example.k3senv.io",
				"names": map[string]interface{}{
					"plural": "sampleresources",
				},
			},
		},
	}

	crds := []unstructured.Unstructured{*crdWithoutKind}

	_, err = resources.FilterConvertibleCRDs(scheme, crds)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("missing required field: spec.names.kind"))
}

func TestFilterConvertibleCRDs_MultipleConvertible(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())
	err = v1beta1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	crd1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "sampleresources.example.k3senv.io",
			},
			"spec": map[string]interface{}{
				"group": "example.k3senv.io",
				"names": map[string]interface{}{
					"kind":   "SampleResource",
					"plural": "sampleresources",
				},
			},
		},
	}

	crd2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "examples.example.k3senv.io",
			},
			"spec": map[string]interface{}{
				"group": "example.k3senv.io",
				"names": map[string]interface{}{
					"kind":   "SampleResource",
					"plural": "examples",
				},
			},
		},
	}

	nonConvertible := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "others.other.io",
			},
			"spec": map[string]interface{}{
				"group": "other.io",
				"names": map[string]interface{}{
					"kind":   "Other",
					"plural": "others",
				},
			},
		},
	}

	crds := []unstructured.Unstructured{*crd1, *nonConvertible, *crd2}

	result, err := resources.FilterConvertibleCRDs(scheme, crds)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(HaveLen(2))
	g.Expect(result[0].GetName()).To(Equal("sampleresources.example.k3senv.io"))
	g.Expect(result[1].GetName()).To(Equal("examples.example.k3senv.io"))
}
