package k3senv

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/itchyny/gojq"
	"github.com/lburgazzoli/k3s-envtest/internal/gvk"
	"github.com/lburgazzoli/k3s-envtest/internal/resources"
	"github.com/lburgazzoli/k3s-envtest/internal/testutil"
	"github.com/mdelapenya/tlscert"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/conversion"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

type CertData struct {
	CACert     []byte
	ServerCert []byte
	ServerKey  []byte
}

func (cd *CertData) CABundle() []byte {
	return []byte(base64.StdEncoding.EncodeToString(cd.CACert))
}

func readFile(path string, elements ...string) ([]byte, error) {
	pathElements := []string{path}
	pathElements = append(pathElements, elements...)
	fullPath := filepath.Join(pathElements...)

	// filepath.Join cleans the path
	//
	//nolint:gosec
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", fullPath, err)
	}
	return data, nil
}

func generateCertificates(
	certPath string,
	validity time.Duration,
) (*CertData, error) {
	if err := os.MkdirAll(certPath, 0750); err != nil {
		return nil, fmt.Errorf("failed to create cert directory: %w", err)
	}

	caCert := tlscert.SelfSignedFromRequest(tlscert.Request{
		Name:      "ca",
		Host:      "k3senv-ca",
		ValidFor:  validity,
		IsCA:      true,
		ParentDir: certPath,
	})

	serverCert := tlscert.SelfSignedFromRequest(tlscert.Request{
		Name:      "tls",
		Host:      strings.Join(CertificateSANs, ","),
		ValidFor:  validity,
		Parent:    caCert,
		ParentDir: certPath,
	})

	if caCert == nil || serverCert == nil {
		return nil, errors.New("failed to generate certificates")
	}

	caCertPEM, err := readFile(certPath, CACertFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}

	serverCertPEM, err := readFile(certPath, CertFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read server cert: %w", err)
	}

	serverKeyPEM, err := readFile(certPath, KeyFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read server key: %w", err)
	}

	return &CertData{
		CACert:     caCertPEM,
		ServerCert: serverCertPEM,
		ServerKey:  serverKeyPEM,
	}, nil
}

func determineConvertibleCRDs(
	crds []unstructured.Unstructured,
	scheme *runtime.Scheme,
) ([]unstructured.Unstructured, error) {
	convertibles := map[schema.GroupKind]struct{}{}
	for gvk := range scheme.AllKnownTypes() {
		obj, err := scheme.New(gvk)
		if err != nil {
			return nil, fmt.Errorf("failed to create a new API object for %s, %w", gvk, err)
		}
		if ok, err := conversion.IsConvertible(scheme, obj); ok && err == nil {
			convertibles[gvk.GroupKind()] = struct{}{}
		}
	}

	var convertibleCRDs []unstructured.Unstructured
	for _, crd := range crds {
		group, found, err := unstructured.NestedString(crd.Object, "spec", "group")
		if err != nil || !found {
			return nil, fmt.Errorf("failed to extract group from CRD %s: %w", crd.GetName(), err)
		}

		kind, found, err := unstructured.NestedString(crd.Object, "spec", "names", "kind")
		if err != nil || !found {
			return nil, fmt.Errorf("failed to extract kind from CRD %s: %w", crd.GetName(), err)
		}

		if _, ok := convertibles[schema.GroupKind{Group: group, Kind: kind}]; ok {
			convertibleCRDs = append(convertibleCRDs, crd)
		}
	}

	return convertibleCRDs, nil
}

func (e *K3sEnv) checkWebhookHealth(
	ctx context.Context,
	hostPort string,
) error {
	client := &http.Client{
		Timeout: e.options.Webhook.HealthCheckTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				//nolint:gosec
				InsecureSkipVerify: true,
			},
		},
	}

	url := fmt.Sprintf("https://%s/", hostPort)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to webhook server: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("webhook server returned error status: %d", resp.StatusCode)
	}

	return nil
}

func ApplyJQTransform(
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

func extractNames(objs []unstructured.Unstructured) []string {
	names := make([]string, len(objs))
	for i := range objs {
		names[i] = objs[i].GetName()
	}
	return names
}

func loadManifestFromFile(
	scheme *runtime.Scheme,
	filePath string,
) ([]unstructured.Unstructured, error) {
	decoder := serializer.NewCodecFactory(scheme).UniversalDeserializer()

	data, err := readFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	manifests, err := resources.Decode(decoder, data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode YAML from %s: %w", filePath, err)
	}

	var result []unstructured.Unstructured
	for i := range manifests {
		gvkType := manifests[i].GroupVersionKind()
		if gvkType == gvk.CustomResourceDefinition ||
			gvkType == gvk.MutatingWebhookConfiguration ||
			gvkType == gvk.ValidatingWebhookConfiguration {
			result = append(result, manifests[i])
		}
	}

	return result, nil
}

func loadManifestsFromDirFlat(
	scheme *runtime.Scheme,
	dir string,
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
		manifests, err := loadManifestFromFile(scheme, filePath)
		if err != nil {
			return nil, err
		}
		result = append(result, manifests...)
	}

	return result, nil
}

func loadManifestPath(
	scheme *runtime.Scheme,
	path string,
) ([]unstructured.Unstructured, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("manifest path does not exist: %s", path)
		}
		return nil, fmt.Errorf("failed to access manifest path %s: %w", path, err)
	}

	if info.IsDir() {
		return loadManifestsFromDirFlat(scheme, path)
	}

	return loadManifestFromFile(scheme, path)
}

func loadObjectsToManifests(
	scheme *runtime.Scheme,
	objects []client.Object,
) ([]unstructured.Unstructured, error) {
	result := make([]unstructured.Unstructured, 0, len(objects))
	for _, obj := range objects {
		if err := resources.EnsureGroupVersionKind(scheme, obj); err != nil {
			return nil, fmt.Errorf("failed to ensure GVK for object %T: %w", obj, err)
		}

		u, err := resources.ToUnstructured(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to convert object to unstructured: %w", err)
		}

		result = append(result, *u.DeepCopy())
	}

	return result, nil
}

func loadManifestsFromPaths(
	scheme *runtime.Scheme,
	paths []string,
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

		if strings.ContainsAny(resolvedPath, "*?[]") {
			matches, err := filepath.Glob(resolvedPath)
			if err != nil {
				return nil, fmt.Errorf("invalid glob pattern %s: %w", resolvedPath, err)
			}
			if len(matches) == 0 {
				return nil, fmt.Errorf("glob pattern matched no files: %s", resolvedPath)
			}

			for _, match := range matches {
				manifests, err := loadManifestPath(scheme, match)
				if err != nil {
					return nil, err
				}
				result = append(result, manifests...)
			}
		} else {
			manifests, err := loadManifestPath(scheme, resolvedPath)
			if err != nil {
				return nil, err
			}
			result = append(result, manifests...)
		}
	}

	return result, nil
}
