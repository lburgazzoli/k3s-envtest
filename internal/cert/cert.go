package cert

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mdelapenya/tlscert"
)

const (
	// CACertFileName is the filename for the CA certificate PEM file.
	CACertFileName = "cert-ca.pem"

	// CertFileName is the filename for the TLS certificate PEM file.
	CertFileName = "cert-tls.pem"

	// KeyFileName is the filename for the TLS private key PEM file.
	KeyFileName = "key-tls.pem"

	// DefaultDirPermission is the default permission for certificate directories.
	DefaultDirPermission = 0o750
)

// Data contains the certificate and key data in PEM format.
type Data struct {
	CACert     []byte
	ServerCert []byte
	ServerKey  []byte
}

// CABundle returns the CA certificate as a base64-encoded string.
func (d *Data) CABundle() []byte {
	return []byte(base64.StdEncoding.EncodeToString(d.CACert))
}

// New generates TLS certificates in the specified path with the given validity and SANs.
// Returns the certificate data in PEM format.
func New(path string, validity time.Duration, sans []string) (*Data, error) {
	if err := os.MkdirAll(path, DefaultDirPermission); err != nil {
		return nil, fmt.Errorf("failed to create cert directory: %w", err)
	}

	caCert := tlscert.SelfSignedFromRequest(tlscert.Request{
		Name:      "ca",
		Host:      "k3senv-ca",
		ValidFor:  validity,
		IsCA:      true,
		ParentDir: path,
	})

	serverCert := tlscert.SelfSignedFromRequest(tlscert.Request{
		Name:      "tls",
		Host:      strings.Join(sans, ","),
		ValidFor:  validity,
		Parent:    caCert,
		ParentDir: path,
	})

	if caCert == nil || serverCert == nil {
		return nil, errors.New("failed to generate certificates")
	}

	caCertPEM, err := readFile(path, CACertFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}

	serverCertPEM, err := readFile(path, CertFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read server cert: %w", err)
	}

	serverKeyPEM, err := readFile(path, KeyFileName)
	if err != nil {
		return nil, fmt.Errorf("failed to read server key: %w", err)
	}

	return &Data{
		CACert:     caCertPEM,
		ServerCert: serverCertPEM,
		ServerKey:  serverKeyPEM,
	}, nil
}

func readFile(path string, elements ...string) ([]byte, error) {
	pathElements := []string{path}
	pathElements = append(pathElements, elements...)
	fullPath := filepath.Join(pathElements...)

	//nolint:gosec // filepath.Join cleans the path
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", fullPath, err)
	}
	return data, nil
}
