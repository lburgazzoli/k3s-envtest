package k3senv_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lburgazzoli/k3s-envtest/pkg/k3senv"
)

const testCertDir = "/tmp/certs"

func TestOptions_FunctionalStyle(t *testing.T) {
	scheme := runtime.NewScheme()

	env, err := k3senv.New(
		k3senv.WithScheme(scheme),
		k3senv.WithCertDir(testCertDir),
		k3senv.WithManifest("/path/to/manifests1"),
		k3senv.WithManifests("/path/to/manifests2", "/path/to/manifests3"),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Scheme() != scheme {
		t.Error("scheme not set correctly")
	}

	if env.CertDir() != testCertDir {
		t.Errorf("certDir = %q, want %q", env.CertDir(), testCertDir)
	}
}

func TestOptions_StructStyle(t *testing.T) {
	scheme := runtime.NewScheme()

	env, err := k3senv.New(&k3senv.Options{
		Scheme:    scheme,
		CertDir:   testCertDir,
		Manifests: []string{"/path/to/manifests1", "/path/to/manifests2"},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Scheme() != scheme {
		t.Error("scheme not set correctly")
	}

	if env.CertDir() != testCertDir {
		t.Errorf("certDir = %q, want %q", env.CertDir(), testCertDir)
	}
}

func TestOptions_MixedStyle(t *testing.T) {
	scheme := runtime.NewScheme()

	env, err := k3senv.New(
		&k3senv.Options{
			Scheme:    scheme,
			Manifests: []string{"/path/to/manifests1"},
		},
		k3senv.WithCertDir(testCertDir),
		k3senv.WithManifest("/path/to/manifests2"),
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if env.Scheme() != scheme {
		t.Error("scheme not set correctly")
	}

	if env.CertDir() != testCertDir {
		t.Errorf("certDir = %q, want %q", env.CertDir(), testCertDir)
	}
}

func TestOptions_ApplyToOptions(t *testing.T) {
	scheme := runtime.NewScheme()

	opt1 := &k3senv.Options{
		Scheme:  scheme,
		CertDir: testCertDir,
	}

	opt2 := &k3senv.Options{
		Manifests: []string{"/path/to/manifests"},
	}

	target := &k3senv.Options{}
	opt1.ApplyToOptions(target)
	opt2.ApplyToOptions(target)

	if target.Scheme != scheme {
		t.Error("scheme not applied correctly")
	}

	if target.CertDir != testCertDir {
		t.Errorf("certDir = %q, want %q", target.CertDir, testCertDir)
	}

	if len(target.Manifests) != 1 {
		t.Errorf("len(manifests) = %d, want 1", len(target.Manifests))
	}
}
