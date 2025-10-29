package webhook_test

import (
	"testing"

	"github.com/lburgazzoli/k3s-envtest/internal/webhook"

	. "github.com/onsi/gomega"
)

func TestNewClient_WithOptions(t *testing.T) {
	g := NewWithT(t)

	// Test that client can be created with no options (options are applied successfully)
	client, err := webhook.NewClient("localhost", 9443)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(client).NotTo(BeNil())
}

func TestNewClient_StructStyle(t *testing.T) {
	g := NewWithT(t)

	// Test struct-style options with empty options
	client, err := webhook.NewClient("localhost", 9443, &webhook.ClientOptions{})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(client).NotTo(BeNil())
}

func TestNewClient_MixedStyle(t *testing.T) {
	g := NewWithT(t)

	// Test that mixed styles work (struct + functional)
	client, err := webhook.NewClient("localhost", 9443,
		&webhook.ClientOptions{},
	)

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(client).NotTo(BeNil())
}

func TestNewClient_InvalidCACert(t *testing.T) {
	g := NewWithT(t)

	client, err := webhook.NewClient("localhost", 9443, webhook.WithClientCACert([]byte("invalid cert")))
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to parse CA certificate"))
	g.Expect(client).To(BeNil())
}
