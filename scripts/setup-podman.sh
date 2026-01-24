#!/bin/bash
# Setup script for running k3s-envtest tests with Podman
# See: https://golang.testcontainers.org/system_requirements/using_podman/

set -e

echo "Setting up Podman for k3s-envtest tests..."

# Detect the Podman socket path
if command -v podman &> /dev/null && podman machine inspect &> /dev/null; then
    PODMAN_SOCKET=$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')
    export DOCKER_HOST="unix://${PODMAN_SOCKET}"
elif [ -S "${XDG_RUNTIME_DIR}/podman/podman.sock" ]; then
    export DOCKER_HOST="unix://${XDG_RUNTIME_DIR}/podman/podman.sock"
else
    echo "Warning: Could not detect Podman socket path"
fi

# Disable Ryuk for Podman (often needed due to container management differences)
export TESTCONTAINERS_RYUK_DISABLED=true

echo ""
echo "Podman setup complete!"
echo ""
echo "Environment variables set:"
echo "  DOCKER_HOST=${DOCKER_HOST}"
echo "  TESTCONTAINERS_RYUK_DISABLED=${TESTCONTAINERS_RYUK_DISABLED}"
echo ""
echo "To run tests, use:"
echo "  source scripts/setup-podman.sh"
echo "  go test ./pkg/k3senv/..."
echo ""
echo "Or manually export the variables:"
echo "  export DOCKER_HOST=unix://\$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')"
echo "  export TESTCONTAINERS_RYUK_DISABLED=true"
echo "  go test ./pkg/k3senv/..."
