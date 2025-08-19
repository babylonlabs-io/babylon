package tmanager

import (
	"fmt"
	"net"
	"strconv"
	"sync"
)

const (
	// Port range for dynamic allocation
	DefaultMinPort = 20000
	DefaultMaxPort = 30000
)

// PortManager manages dynamic port allocation
type PortManager struct {
	UsedPorts map[int]bool
	Mutex     sync.RWMutex
	MinPort   int
	MaxPort   int
}

// NodePorts holds all port assignments for a node
type NodePorts struct {
	RPC    int // 26657
	P2P    int // 26656
	GRPC   int // 9090
	REST   int // 1317
	EVMRPC int // 8545
	EVMWS  int // 8546
}

// NewPortManager creates a new port manager with default range
func NewPortManager() *PortManager {
	return &PortManager{
		UsedPorts: make(map[int]bool),
		MinPort:   DefaultMinPort,
		MaxPort:   DefaultMaxPort,
	}
}

// AllocatePort allocates a single available port
func (pm *PortManager) AllocatePort() (int, error) {
	pm.Mutex.Lock()
	defer pm.Mutex.Unlock()

	for port := pm.MinPort; port <= pm.MaxPort; port++ {
		if !pm.UsedPorts[port] && pm.isPortAvailable(port) {
			pm.UsedPorts[port] = true
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", pm.MinPort, pm.MaxPort)
}

// AllocatePortRange allocates a consecutive range of ports
func (pm *PortManager) AllocatePortRange(count int) ([]int, error) {
	pm.Mutex.Lock()
	defer pm.Mutex.Unlock()

	for startPort := pm.MinPort; startPort <= pm.MaxPort-count+1; startPort++ {
		// Check if we can allocate 'count' consecutive ports starting from startPort
		available := true
		for i := 0; i < count; i++ {
			port := startPort + i
			if pm.UsedPorts[port] || !pm.isPortAvailable(port) {
				available = false
				break
			}
		}

		if available {
			ports := make([]int, count)
			for i := 0; i < count; i++ {
				port := startPort + i
				pm.UsedPorts[port] = true
				ports[i] = port
			}
			return ports, nil
		}
	}

	return nil, fmt.Errorf("cannot allocate %d consecutive ports in range %d-%d", count, pm.MinPort, pm.MaxPort)
}

// ReleasePort releases a port back to the available pool
func (pm *PortManager) ReleasePort(port int) {
	pm.Mutex.Lock()
	defer pm.Mutex.Unlock()
	delete(pm.UsedPorts, port)
}

// ReleasePorts releases multiple ports
func (pm *PortManager) ReleasePorts(ports []int) {
	pm.Mutex.Lock()
	defer pm.Mutex.Unlock()
	for _, port := range ports {
		delete(pm.UsedPorts, port)
	}
}

// isPortAvailable checks if a port is actually available on the system
func (pm *PortManager) isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// AllocateNodePorts allocates all required ports for a node
func (pm *PortManager) AllocateNodePorts() (*NodePorts, error) {
	ports, err := pm.AllocatePortRange(6)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate node ports: %w", err)
	}

	return &NodePorts{
		RPC:    ports[0],
		P2P:    ports[1],
		GRPC:   ports[2],
		REST:   ports[3],
		EVMRPC: ports[4],
		EVMWS:  ports[5],
	}, nil
}

// ReleaseNodePorts releases all ports used by a node
func (pm *PortManager) ReleaseNodePorts(ports *NodePorts) {
	if ports == nil {
		return
	}
	pm.ReleasePorts([]int{ports.RPC, ports.P2P, ports.GRPC, ports.REST, ports.EVMRPC, ports.EVMWS})
}

func (np *NodePorts) ContainerExposedPorts() []string {
	return []string{
		strconv.FormatInt(int64(np.RPC), 10),
		strconv.FormatInt(int64(np.P2P), 10),
		strconv.FormatInt(int64(np.GRPC), 10),
		strconv.FormatInt(int64(np.REST), 10),
		strconv.FormatInt(int64(np.EVMRPC), 10),
		strconv.FormatInt(int64(np.EVMWS), 10),
	}
}
