package resources_test

import (
	"context"
	"testing"
	"time"

	"github.com/lburgazzoli/k3s-envtest/internal/resources"
	"github.com/lburgazzoli/k3s-envtest/internal/testdata/v1alpha1"
	"github.com/lburgazzoli/k3s-envtest/internal/testdata/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/gomega"
)

func TestFilterConvertibleCRDs_Success(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())
	err = v1beta1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	convertibleCRD := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sampleresources.example.k3senv.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "example.k3senv.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "SampleResource",
				Plural: "sampleresources",
			},
		},
	}

	nonConvertibleCRD := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "others.other.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "other.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "Other",
				Plural: "others",
			},
		},
	}

	crds := []apiextensionsv1.CustomResourceDefinition{convertibleCRD, nonConvertibleCRD}

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

	result, err := resources.FilterConvertibleCRDs(scheme, []apiextensionsv1.CustomResourceDefinition{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(BeEmpty())
}

func TestFilterConvertibleCRDs_NonConvertible(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	nonConvertibleCRD := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "others.other.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "other.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "Other",
				Plural: "others",
			},
		},
	}

	crds := []apiextensionsv1.CustomResourceDefinition{nonConvertibleCRD}

	result, err := resources.FilterConvertibleCRDs(scheme, crds)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(BeEmpty())
}

func TestFilterConvertibleCRDs_MissingGroup(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	crdWithoutGroup := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sampleresources.example.k3senv.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "", // Missing group
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "SampleResource",
				Plural: "sampleresources",
			},
		},
	}

	crds := []apiextensionsv1.CustomResourceDefinition{crdWithoutGroup}

	_, err = resources.FilterConvertibleCRDs(scheme, crds)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("missing required field: spec.group"))
}

func TestFilterConvertibleCRDs_MissingKind(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	err := v1alpha1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	crdWithoutKind := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sampleresources.example.k3senv.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "example.k3senv.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "", // Missing kind
				Plural: "sampleresources",
			},
		},
	}

	crds := []apiextensionsv1.CustomResourceDefinition{crdWithoutKind}

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

	crd1 := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "sampleresources.example.k3senv.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "example.k3senv.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "SampleResource",
				Plural: "sampleresources",
			},
		},
	}

	crd2 := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "examples.example.k3senv.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "example.k3senv.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "SampleResource",
				Plural: "examples",
			},
		},
	}

	nonConvertible := apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "others.other.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "other.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "Other",
				Plural: "others",
			},
		},
	}

	crds := []apiextensionsv1.CustomResourceDefinition{crd1, nonConvertible, crd2}

	result, err := resources.FilterConvertibleCRDs(scheme, crds)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(HaveLen(2))
	g.Expect(result[0].GetName()).To(Equal("sampleresources.example.k3senv.io"))
	g.Expect(result[1].GetName()).To(Equal("examples.example.k3senv.io"))
}

func TestIsCRDEstablished_True(t *testing.T) {
	g := NewWithT(t)

	crd := &apiextensionsv1.CustomResourceDefinition{
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
				{
					Type:   apiextensionsv1.Established,
					Status: apiextensionsv1.ConditionTrue,
				},
			},
		},
	}

	g.Expect(resources.IsCRDEstablished(crd)).To(BeTrue())
}

func TestIsCRDEstablished_False(t *testing.T) {
	g := NewWithT(t)

	crd := &apiextensionsv1.CustomResourceDefinition{
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
				{
					Type:   apiextensionsv1.NamesAccepted,
					Status: apiextensionsv1.ConditionTrue,
				},
			},
		},
	}

	g.Expect(resources.IsCRDEstablished(crd)).To(BeFalse())
}

func TestIsCRDEstablished_NoConditions(t *testing.T) {
	g := NewWithT(t)

	crd := &apiextensionsv1.CustomResourceDefinition{
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{},
		},
	}

	g.Expect(resources.IsCRDEstablished(crd)).To(BeFalse())
}

func TestWaitForCRDEstablished_AlreadyEstablished(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test.example.com",
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
				{
					Type:   apiextensionsv1.Established,
					Status: apiextensionsv1.ConditionTrue,
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	err := apiextensionsv1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	cli := &fakeCRDClient{crd: crd}

	err = resources.WaitForCRDEstablished(ctx, cli, "test.example.com", time.Millisecond, time.Second)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestWaitForCRDEstablished_Timeout(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test.example.com",
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{},
		},
	}

	scheme := runtime.NewScheme()
	err := apiextensionsv1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	cli := &fakeCRDClient{crd: crd}

	err = resources.WaitForCRDEstablished(ctx, cli, "test.example.com", time.Millisecond, 10*time.Millisecond)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not established"))
}

type fakeCRDClient struct {
	crd *apiextensionsv1.CustomResourceDefinition
}

func (f *fakeCRDClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	if crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition); ok {
		f.crd.DeepCopyInto(crd)
		return nil
	}
	return nil
}

func (f *fakeCRDClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

func (f *fakeCRDClient) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	return nil
}

func (f *fakeCRDClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	return nil
}

func (f *fakeCRDClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	return nil
}

func (f *fakeCRDClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

func (f *fakeCRDClient) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

func (f *fakeCRDClient) Status() client.SubResourceWriter {
	return nil
}

func (f *fakeCRDClient) SubResource(subResource string) client.SubResourceClient {
	return nil
}

func (f *fakeCRDClient) Scheme() *runtime.Scheme {
	return nil
}

func (f *fakeCRDClient) RESTMapper() meta.RESTMapper {
	return nil
}

func (f *fakeCRDClient) GroupVersionKindFor(obj runtime.Object) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (f *fakeCRDClient) IsObjectNamespaced(obj runtime.Object) (bool, error) {
	return false, nil
}
