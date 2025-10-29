package k3senv

import (
	"context"
	"fmt"

	"github.com/lburgazzoli/k3s-envtest/internal/resources"
	"github.com/lburgazzoli/k3s-envtest/internal/webhook"
	"sigs.k8s.io/controller-runtime/pkg/client"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/utils/ptr"
)

func (e *K3sEnv) installWebhook(
	ctx context.Context,
	webhook client.Object,
	baseURL string,
	caBundle string,
) error {
	switch wh := webhook.(type) {
	case *admissionregistrationv1.MutatingWebhookConfiguration:
		resources.PatchMutatingWebhookConfiguration(wh, baseURL, caBundle)
	case *admissionregistrationv1.ValidatingWebhookConfiguration:
		resources.PatchValidatingWebhookConfiguration(wh, baseURL, caBundle)
	default:
		return fmt.Errorf("unsupported webhook type: %T", webhook)
	}

	if err := resources.EnsureGroupVersionKind(e.options.Scheme, webhook); err != nil {
		return fmt.Errorf("failed to set GVK for webhook %s: %w", webhook.GetName(), err)
	}

	err := e.cli.Patch(ctx, webhook, client.Apply, client.ForceOwnership, client.FieldOwner("k3s-envtest"))
	if err != nil {
		return fmt.Errorf("failed to apply webhook %s: %w", webhook.GetName(), err)
	}

	e.debugf("Webhook configuration %s applied", webhook.GetName())

	if !ptr.Deref(e.options.Webhook.CheckReadiness, false) {
		return nil
	}

	if err := e.waitForWebhookEndpointsReady(ctx, webhook, e.options.Webhook.Port); err != nil {
		return fmt.Errorf("webhook config %s endpoints not ready: %w", webhook.GetName(), err)
	}

	return nil
}

func (e *K3sEnv) installWebhooks(
	ctx context.Context,
	hostPort string,
) error {
	baseURL := fmt.Sprintf("%s://%s", WebhookURLScheme, hostPort)
	caBundle := string(e.certData.CABundle())

	mutating := e.MutatingWebhookConfigurations()
	for i := range mutating {
		if err := e.installWebhook(ctx, &mutating[i], baseURL, caBundle); err != nil {
			return err
		}
	}

	validating := e.ValidatingWebhookConfigurations()
	for i := range validating {
		if err := e.installWebhook(ctx, &validating[i], baseURL, caBundle); err != nil {
			return err
		}
	}

	return nil
}

func (e *K3sEnv) waitForWebhookEndpointsReady(
	ctx context.Context,
	webhookConfig client.Object,
	port int,
) error {
	webhookURLs, err := resources.ExtractWebhookURLs(webhookConfig)
	if err != nil {
		return fmt.Errorf("failed to extract webhook URLs: %w", err)
	}

	if len(webhookURLs) == 0 {
		e.debugf("No webhook endpoints found in config %s, skipping health check", webhookConfig.GetName())
		return nil
	}

	e.debugf("Checking %d webhook endpoints for %s...", len(webhookURLs), webhookConfig.GetName())

	webhookClient, err := webhook.NewClient(
		"127.0.0.1",
		port,
		webhook.WithClientCACert(e.certData.CACert),
	)
	if err != nil {
		return fmt.Errorf("failed to create webhook client: %w", err)
	}

	if err := webhookClient.WaitForEndpoints(
		ctx,
		webhookURLs,
		webhook.WithPollInterval(e.options.Webhook.PollInterval),
		webhook.WithReadyTimeout(e.options.Webhook.ReadyTimeout),
		webhook.WithWaitCallTimeout(e.options.Webhook.HealthCheckTimeout),
	); err != nil {
		return fmt.Errorf("webhook endpoints not ready: %w", err)
	}

	e.debugf("All webhook endpoints for %s are ready", webhookConfig.GetName())

	return nil
}
