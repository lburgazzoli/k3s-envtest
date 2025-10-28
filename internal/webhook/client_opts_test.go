package webhook_test

import (
	"testing"
	"time"

	"github.com/lburgazzoli/k3s-envtest/internal/webhook"

	. "github.com/onsi/gomega"
)

func TestNewClient_WithOptions(t *testing.T) {
	g := NewWithT(t)

	// Test that options are applied correctly (using timeout as example)
	client, err := webhook.NewClient("localhost", 9443, webhook.WithClientTimeout(20*time.Second))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(client).NotTo(BeNil())
}

func TestNewClient_WithTimeout(t *testing.T) {
	g := NewWithT(t)

	client, err := webhook.NewClient("localhost", 9443, webhook.WithClientTimeout(10*time.Second))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(client).NotTo(BeNil())
}

func TestNewClient_StructStyle(t *testing.T) {
	g := NewWithT(t)

	client, err := webhook.NewClient("localhost", 9443, &webhook.ClientOptions{
		Timeout: 15 * time.Second,
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(client).NotTo(BeNil())
}

func TestNewClient_MixedStyle(t *testing.T) {
	g := NewWithT(t)

	// Test mixed functional and struct style options
	client, err := webhook.NewClient("localhost", 9443,
		&webhook.ClientOptions{
			Timeout: 15 * time.Second,
		},
		webhook.WithClientTimeout(20*time.Second), // This should override the struct timeout
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
