package resources

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lburgazzoli/k3s-envtest/internal/gvk"
	"github.com/lburgazzoli/k3s-envtest/internal/resources/filter"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const testMultiDocYAML = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: test-crd
---
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
`

const testCRDYAML = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: crd1
`

const testPodYAML = `apiVersion: v1
kind: Pod
metadata:
  name: pod1
`

const testInvalidYAML = `invalid: [yaml content`

func TestLoadFromFile_Success(t *testing.T) {
	g := NewWithT(t)

	// Create temporary YAML file
	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "test.yaml")

	err := os.WriteFile(yamlFile, []byte(testMultiDocYAML), 0o600)
	g.Expect(err).NotTo(HaveOccurred())

	// Load without filter
	manifests, err := loadFromFile(yamlFile, nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(manifests).To(HaveLen(2))

	// Load with filter
	objectFilter := filter.ByType(gvk.CustomResourceDefinition)
	manifests, err = loadFromFile(yamlFile, objectFilter)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(manifests).To(HaveLen(1))
	g.Expect(manifests[0].GetName()).To(Equal("test-crd"))
}

func TestLoadFromFile_FileNotFound(t *testing.T) {
	g := NewWithT(t)

	_, err := loadFromFile("/nonexistent/file.yaml", nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to read file"))
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "invalid.yaml")

	err := os.WriteFile(yamlFile, []byte(testInvalidYAML), 0o600)
	g.Expect(err).NotTo(HaveOccurred())

	_, err = loadFromFile(yamlFile, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to decode YAML"))
}

func TestLoadFromDirectory_Success(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()

	// Create multiple YAML files
	yaml1 := filepath.Join(tmpDir, "file1.yaml")
	yaml2 := filepath.Join(tmpDir, "file2.yml")
	txtFile := filepath.Join(tmpDir, "ignore.txt")
	subDir := filepath.Join(tmpDir, "subdir")

	err := os.WriteFile(yaml1, []byte(testCRDYAML), 0o600)
	g.Expect(err).NotTo(HaveOccurred())
	err = os.WriteFile(yaml2, []byte(testPodYAML), 0o600)
	g.Expect(err).NotTo(HaveOccurred())
	err = os.WriteFile(txtFile, []byte("ignored"), 0o600)
	g.Expect(err).NotTo(HaveOccurred())
	err = os.Mkdir(subDir, 0o755)
	g.Expect(err).NotTo(HaveOccurred())

	// Load without filter
	manifests, err := loadFromDirectory(tmpDir, nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(manifests).To(HaveLen(2))

	// Load with filter
	objectFilter := filter.ByType(gvk.CustomResourceDefinition)
	manifests, err = loadFromDirectory(tmpDir, objectFilter)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(manifests).To(HaveLen(1))
	g.Expect(manifests[0].GetName()).To(Equal("crd1"))
}

func TestLoadFromDirectory_DirectoryNotFound(t *testing.T) {
	g := NewWithT(t)

	_, err := loadFromDirectory("/nonexistent/dir", nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to read directory"))
}

func TestLoadFromPath_File(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "test.yaml")

	err := os.WriteFile(yamlFile, []byte(testPodYAML), 0o600)
	g.Expect(err).NotTo(HaveOccurred())

	manifests, err := loadFromPath(yamlFile, nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(manifests).To(HaveLen(1))
}

func TestLoadFromPath_Directory(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	yamlFile := filepath.Join(tmpDir, "test.yaml")

	err := os.WriteFile(yamlFile, []byte(testPodYAML), 0o600)
	g.Expect(err).NotTo(HaveOccurred())

	manifests, err := loadFromPath(tmpDir, nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(manifests).To(HaveLen(1))
}

func TestLoadFromPath_NotFound(t *testing.T) {
	g := NewWithT(t)

	_, err := loadFromPath("/nonexistent/path", nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("does not exist"))
}

func TestUnstructuredFromObjects_Success(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	err := apiextensionsv1.AddToScheme(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-crd",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "example.com",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   "Example",
				Plural: "examples",
			},
		},
	}

	objects := []client.Object{crd}

	// Without filter
	manifests, err := UnstructuredFromObjects(scheme, objects, nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(manifests).To(HaveLen(1))
	g.Expect(manifests[0].GetName()).To(Equal("test-crd"))

	// With filter that matches
	objectFilter := filter.ByType(gvk.CustomResourceDefinition)
	manifests, err = UnstructuredFromObjects(scheme, objects, objectFilter)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(manifests).To(HaveLen(1))

	// With filter that doesn't match
	podGVK := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
	objectFilter = filter.ByType(podGVK)
	manifests, err = UnstructuredFromObjects(scheme, objects, objectFilter)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(manifests).To(HaveLen(0))
}

func TestUnstructuredFromObjects_EmptyList(t *testing.T) {
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	objects := []client.Object{}

	manifests, err := UnstructuredFromObjects(scheme, objects, nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(manifests).To(HaveLen(0))
}
