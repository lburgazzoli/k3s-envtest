package v1alpha1

import (
	"github.com/lburgazzoli/k3s-envtest/internal/testdata/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	Group   = "example.k3senv.io"
	Version = "v1alpha1"
)

var (
	GroupVersion  = schema.GroupVersion{Group: Group, Version: Version}
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		GroupVersion,
		&SampleResource{},
		&SampleResourceList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}

type SampleResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SampleResourceSpec   `json:"spec,omitempty"`
	Status SampleResourceStatus `json:"status,omitempty"`
}

type SampleResourceSpec struct {
	FieldAlpha string `json:"fieldAlpha,omitempty"`
}

type SampleResourceStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type SampleResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SampleResource `json:"items"`
}

func (r *SampleResource) DeepCopyObject() runtime.Object {
	if r == nil {
		return nil
	}
	out := new(SampleResource)
	r.DeepCopyInto(out)
	return out
}

func (r *SampleResource) DeepCopyInto(out *SampleResource) {
	*out = *r
	out.TypeMeta = r.TypeMeta
	r.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = r.Spec
	if r.Status.Conditions != nil {
		out.Status.Conditions = make([]metav1.Condition, len(r.Status.Conditions))
		for i := range r.Status.Conditions {
			r.Status.Conditions[i].DeepCopyInto(&out.Status.Conditions[i])
		}
	}
}

func (r *SampleResourceList) DeepCopyObject() runtime.Object {
	if r == nil {
		return nil
	}
	out := new(SampleResourceList)
	r.DeepCopyInto(out)
	return out
}

func (r *SampleResourceList) DeepCopyInto(out *SampleResourceList) {
	*out = *r
	out.TypeMeta = r.TypeMeta
	r.ListMeta.DeepCopyInto(&out.ListMeta)
	if r.Items != nil {
		out.Items = make([]SampleResource, len(r.Items))
		for i := range r.Items {
			r.Items[i].DeepCopyInto(&out.Items[i])
		}
	}
}

func (r *SampleResource) ConvertTo(
	dstRaw conversion.Hub,
) error {
	dst := dstRaw.(*v1beta1.SampleResource)

	dst.ObjectMeta = r.ObjectMeta
	dst.Spec.FieldBeta = r.Spec.FieldAlpha
	dst.Status.Conditions = r.Status.Conditions

	return nil
}

func (r *SampleResource) ConvertFrom(
	srcRaw conversion.Hub,
) error {
	src := srcRaw.(*v1beta1.SampleResource)

	r.ObjectMeta = src.ObjectMeta
	r.Spec.FieldAlpha = src.Spec.FieldBeta
	r.Status.Conditions = src.Status.Conditions

	return nil
}
