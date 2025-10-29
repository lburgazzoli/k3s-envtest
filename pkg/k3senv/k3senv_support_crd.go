package k3senv

import (
	"context"
	"fmt"

	"github.com/lburgazzoli/k3s-envtest/internal/resources"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func (e *K3sEnv) installCRDs(ctx context.Context) error {
	crds := e.CustomResourceDefinitions()
	if len(crds) == 0 {
		return nil
	}

	for i := range crds {
		if err := e.InstallCRD(ctx, &crds[i]); err != nil {
			return err
		}
	}

	return nil
}

func (e *K3sEnv) patchAndUpdateCRDConversions(
	ctx context.Context,
	convertibleCRDs []apiextensionsv1.CustomResourceDefinition,
	hostPort string,
) error {
	baseURL := fmt.Sprintf("%s://%s", WebhookURLScheme, hostPort)

	for i := range convertibleCRDs {
		resources.PatchCRDConversion(&convertibleCRDs[i], baseURL, e.certData.CACert)

		if err := e.InstallCRD(ctx, &convertibleCRDs[i]); err != nil {
			return err
		}
	}

	return nil
}
