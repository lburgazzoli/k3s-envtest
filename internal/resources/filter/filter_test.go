//nolint:testpackage // Testing unexported functions
package filter

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/gomega"
)

var (
	testGVKPod = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Pod",
	}
	testGVKService = schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	}
	testGVKDeployment = schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}
)

func makeObject(gvk schema.GroupVersionKind, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	obj.SetName(name)
	return obj
}

func TestByType_SingleGVK(t *testing.T) {
	g := NewWithT(t)

	filter := ByType(testGVKPod)

	pod := makeObject(testGVKPod, "test-pod")
	service := makeObject(testGVKService, "test-service")

	g.Expect(filter(pod)).To(BeTrue())
	g.Expect(filter(service)).To(BeFalse())
}

func TestByType_MultipleGVKs(t *testing.T) {
	g := NewWithT(t)

	filter := ByType(testGVKPod, testGVKService)

	pod := makeObject(testGVKPod, "test-pod")
	service := makeObject(testGVKService, "test-service")
	deployment := makeObject(testGVKDeployment, "test-deployment")

	g.Expect(filter(pod)).To(BeTrue())
	g.Expect(filter(service)).To(BeTrue())
	g.Expect(filter(deployment)).To(BeFalse())
}

func TestByType_NoGVKs(t *testing.T) {
	g := NewWithT(t)

	filter := ByType()

	pod := makeObject(testGVKPod, "test-pod")
	g.Expect(filter(pod)).To(BeFalse())
}

func TestAny_AcceptsAnyMatch(t *testing.T) {
	g := NewWithT(t)

	podFilter := ByType(testGVKPod)
	serviceFilter := ByType(testGVKService)
	anyFilter := Any(podFilter, serviceFilter)

	pod := makeObject(testGVKPod, "test-pod")
	service := makeObject(testGVKService, "test-service")
	deployment := makeObject(testGVKDeployment, "test-deployment")

	g.Expect(anyFilter(pod)).To(BeTrue())
	g.Expect(anyFilter(service)).To(BeTrue())
	g.Expect(anyFilter(deployment)).To(BeFalse())
}

func TestAny_NoFilters(t *testing.T) {
	g := NewWithT(t)

	filter := Any()

	pod := makeObject(testGVKPod, "test-pod")
	g.Expect(filter(pod)).To(BeTrue())
}

func TestAll_RequiresAllMatches(t *testing.T) {
	g := NewWithT(t)

	nameFilter := ObjectFilter(func(obj client.Object) bool {
		return obj.GetName() == "special-pod"
	})
	typeFilter := ByType(testGVKPod)

	allFilter := All(typeFilter, nameFilter)

	specialPod := makeObject(testGVKPod, "special-pod")
	normalPod := makeObject(testGVKPod, "normal-pod")
	service := makeObject(testGVKService, "special-pod")

	g.Expect(allFilter(specialPod)).To(BeTrue())
	g.Expect(allFilter(normalPod)).To(BeFalse())
	g.Expect(allFilter(service)).To(BeFalse())
}

func TestAll_NoFilters(t *testing.T) {
	g := NewWithT(t)

	filter := All()

	pod := makeObject(testGVKPod, "test-pod")
	g.Expect(filter(pod)).To(BeTrue())
}

func TestNegate_InvertsFilter(t *testing.T) {
	g := NewWithT(t)

	podFilter := ByType(testGVKPod)
	notPodFilter := Negate(podFilter)

	pod := makeObject(testGVKPod, "test-pod")
	service := makeObject(testGVKService, "test-service")

	g.Expect(notPodFilter(pod)).To(BeFalse())
	g.Expect(notPodFilter(service)).To(BeTrue())
}

func TestComplexCombination(t *testing.T) {
	g := NewWithT(t)

	// Accept pods OR services, but NOT ones named "excluded"
	typeFilter := ByType(testGVKPod, testGVKService)
	nameFilter := ObjectFilter(func(obj client.Object) bool {
		return obj.GetName() != "excluded"
	})
	complexFilter := All(typeFilter, nameFilter)

	includedPod := makeObject(testGVKPod, "included-pod")
	excludedPod := makeObject(testGVKPod, "excluded")
	includedService := makeObject(testGVKService, "included-service")
	deployment := makeObject(testGVKDeployment, "deployment")

	g.Expect(complexFilter(includedPod)).To(BeTrue())
	g.Expect(complexFilter(excludedPod)).To(BeFalse())
	g.Expect(complexFilter(includedService)).To(BeTrue())
	g.Expect(complexFilter(deployment)).To(BeFalse())
}
