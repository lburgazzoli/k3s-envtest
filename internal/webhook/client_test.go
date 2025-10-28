package webhook_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lburgazzoli/k3s-envtest/internal/webhook"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/gomega"
)

func TestNewClient_Success(t *testing.T) {
	g := NewWithT(t)

	client, err := webhook.NewClient("localhost", 9443)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(client).NotTo(BeNil())
	g.Expect(client.Address()).To(Equal("localhost:9443"))
}

func TestNewClient_EmptyHost(t *testing.T) {
	g := NewWithT(t)

	client, err := webhook.NewClient("", 9443)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("host cannot be empty"))
	g.Expect(client).To(BeNil())
}

func TestNewClient_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero port", 0},
		{"negative port", -1},
		{"port too high", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			client, err := webhook.NewClient("localhost", tt.port)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("invalid port"))
			g.Expect(client).To(BeNil())
		})
	}
}

func TestCall_Success(t *testing.T) {
	g := NewWithT(t)

	// Create test server
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.Method).To(Equal(http.MethodPost))
		g.Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))

		var review admissionv1.AdmissionReview
		err := json.NewDecoder(r.Body).Decode(&review)
		g.Expect(err).NotTo(HaveOccurred())

		// Send back a response
		response := admissionv1.AdmissionReview{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "admission.k8s.io/v1",
				Kind:       "AdmissionReview",
			},
			Response: &admissionv1.AdmissionResponse{
				UID:     review.Request.UID,
				Allowed: true,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client with insecure skip verify (test server uses self-signed cert)
	client, err := webhook.NewClient(server.Listener.Addr().(*net.TCPAddr).IP.String(),
		server.Listener.Addr().(*net.TCPAddr).Port)
	g.Expect(err).NotTo(HaveOccurred())

	// Make request
	review := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID: types.UID("test-uid"),
		},
	}

	resp, err := client.Call(context.Background(), "/validate", review)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resp).NotTo(BeNil())
	g.Expect(resp.Response).NotTo(BeNil())
	g.Expect(resp.Response.Allowed).To(BeTrue())
	g.Expect(resp.Response.UID).To(Equal(types.UID("test-uid")))
}

func TestCall_EmptyPath(t *testing.T) {
	g := NewWithT(t)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.URL.Path).To(Equal("/"))
		response := admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				Allowed: true,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := webhook.NewClient(server.Listener.Addr().(*net.TCPAddr).IP.String(),
		server.Listener.Addr().(*net.TCPAddr).Port)
	g.Expect(err).NotTo(HaveOccurred())

	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: types.UID("test-uid"),
		},
	}

	resp, err := client.Call(context.Background(), "", review)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resp).NotTo(BeNil())
}

func TestCall_ServerError(t *testing.T) {
	g := NewWithT(t)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := webhook.NewClient(server.Listener.Addr().(*net.TCPAddr).IP.String(),
		server.Listener.Addr().(*net.TCPAddr).Port)
	g.Expect(err).NotTo(HaveOccurred())

	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: types.UID("test-uid"),
		},
	}

	resp, err := client.Call(context.Background(), "/validate", review)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("webhook returned server error"))
	g.Expect(resp).To(BeNil())
}

func TestCall_ClientError_Accepted(t *testing.T) {
	g := NewWithT(t)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				Allowed: false,
				Result: &metav1.Status{
					Code:    400,
					Message: "Bad Request",
				},
			},
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client, err := webhook.NewClient(server.Listener.Addr().(*net.TCPAddr).IP.String(),
		server.Listener.Addr().(*net.TCPAddr).Port)
	g.Expect(err).NotTo(HaveOccurred())

	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: types.UID("test-uid"),
		},
	}

	// 4xx errors should not fail the call, just return the response
	resp, err := client.Call(context.Background(), "/validate", review)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resp).NotTo(BeNil())
	g.Expect(resp.Response.Allowed).To(BeFalse())
}

func TestCall_ContextCancelled(t *testing.T) {
	g := NewWithT(t)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	client, err := webhook.NewClient(server.Listener.Addr().(*net.TCPAddr).IP.String(),
		server.Listener.Addr().(*net.TCPAddr).Port)
	g.Expect(err).NotTo(HaveOccurred())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: types.UID("test-uid"),
		},
	}

	resp, err := client.Call(ctx, "/validate", review)
	g.Expect(err).To(HaveOccurred())
	g.Expect(resp).To(BeNil())
}

func TestCall_Timeout(t *testing.T) {
	g := NewWithT(t)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	client, err := webhook.NewClient(server.Listener.Addr().(*net.TCPAddr).IP.String(),
		server.Listener.Addr().(*net.TCPAddr).Port,
		webhook.WithClientTimeout(50*time.Millisecond))
	g.Expect(err).NotTo(HaveOccurred())

	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: types.UID("test-uid"),
		},
	}

	resp, err := client.Call(context.Background(), "/validate", review)
	g.Expect(err).To(HaveOccurred())
	g.Expect(resp).To(BeNil())
}

func TestCall_InvalidResponse(t *testing.T) {
	g := NewWithT(t)

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client, err := webhook.NewClient(server.Listener.Addr().(*net.TCPAddr).IP.String(),
		server.Listener.Addr().(*net.TCPAddr).Port)
	g.Expect(err).NotTo(HaveOccurred())

	review := admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: types.UID("test-uid"),
		},
	}

	resp, err := client.Call(context.Background(), "/validate", review)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to unmarshal"))
	g.Expect(resp).To(BeNil())
}
