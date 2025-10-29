package k3senv

import (
	"fmt"
	"net"
)

// FindAvailablePort finds an available TCP port on the local machine.
//
// The function binds to port 0, which causes the OS to assign any available port,
// then immediately closes the listener and returns the port number.
//
// Note: Go's net.Listen automatically sets SO_REUSEADDR on Unix-like systems,
// which allows the port to be reused even if it's in TIME_WAIT state. However,
// there is a small race condition window between closing the listener and actually
// using the port where another process could grab it. In practice, this is rare.
//
// This is useful for parallel testing where you need unique webhook ports:
//
//	port, err := k3senv.FindAvailablePort()
//	if err != nil {
//	    t.Fatal(err)
//	}
//	env, err := k3senv.New(k3senv.WithWebhookPort(port))
func FindAvailablePort() (int, error) {
	//nolint:noctx // Simple port discovery doesn't require context
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	defer func() {
		_ = listener.Close()
	}()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected address type: %T", listener.Addr())
	}

	return addr.Port, nil
}

// FindAvailablePortInRange finds an available TCP port within the specified range.
//
// This is useful when you need to constrain ports to a specific range, for example
// when firewall rules only allow certain ports.
//
// The function tries each port in the range sequentially until it finds one that's
// available. Returns an error if no port is available in the range.
//
// Example usage:
//
//	// Only use ports allowed by firewall
//	port, err := k3senv.FindAvailablePortInRange(9443, 9543)
//	if err != nil {
//	    t.Skip("No available port in allowed range")
//	}
//	env, err := k3senv.New(k3senv.WithWebhookPort(port))
func FindAvailablePortInRange(minPort int, maxPort int) (int, error) {
	if minPort < 1 || maxPort > 65535 || minPort > maxPort {
		return 0, fmt.Errorf("invalid port range: %d-%d (must be 1-65535 and min <= max)", minPort, maxPort)
	}

	for port := minPort; port <= maxPort; port++ {
		//nolint:noctx // Simple port discovery doesn't require context
		listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			continue // Port not available, try next
		}
		_ = listener.Close()
		return port, nil
	}

	return 0, fmt.Errorf("no available port found in range %d-%d", minPort, maxPort)
}
