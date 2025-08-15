package types

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/ory/dockertest/v3"
)

const (
	// Images that do not have specified tag, latest will be used by default.
	// name of babylon image produced by running `make build-docker`
	BabylonContainerName = "babylonlabs-io/babylond"
)

// ContainerConfig defines configuration for creating a container
type ContainerConfig struct {
	Name        string
	Image       string
	Network     string
	Ports       map[string]int // exposed port -> host port (0 = auto-assign)
	Volumes     []string
	Environment map[string]string
	EntryPoint  []string
	Cmd         []string
	User        string
}

// Container represents a running Docker container
type Container struct {
	Name       string
	Repository string
	Tag        string
}

// ContainerManager manages Docker containers lifecycle
type ContainerManager struct {
	Pool      *dockertest.Pool
	Network   *dockertest.Network
	Resources map[string]*dockertest.Resource
	Mutex     sync.RWMutex
}

// NewContainerManager creates a new container manager
func NewContainerManager(pool *dockertest.Pool, network *dockertest.Network) *ContainerManager {
	return &ContainerManager{
		Pool:      pool,
		Network:   network,
		Resources: make(map[string]*dockertest.Resource),
	}
}

func NewContainerBbnNode(containerName string) *Container {
	return &Container{
		Name:       containerName,
		Repository: BabylonContainerName,
		Tag:        "latest",
	}
}

func (c *Container) ImageName() string {
	return fmt.Sprintf("%s:%s", c.Repository, c.Tag)
}

func (c *Container) ImageExistsLocally() bool {
	return ImageExistsLocally(c.ImageName())
}

func ImageExistsLocally(imageName string) bool {
	cmd := exec.Command("docker", "image", "inspect", imageName)
	return cmd.Run() == nil
}

// CreateContainer creates a new container with the given configuration
// func (cm *ContainerManager) CreateContainer(config *ContainerConfig) (*Container, error) {
// 	cm.Mutex.Lock()
// 	defer cm.Mutex.Unlock()

// 	// Build port bindings for Docker
// 	portBindings := make(map[docker.Port][]docker.PortBinding)
// 	exposedPorts := make([]string, 0, len(config.Ports))

// 	for containerPort, hostPort := range config.Ports {
// 		dockerPort := docker.Port(fmt.Sprintf("%s/tcp", containerPort))
// 		exposedPorts = append(exposedPorts, containerPort)

// 		if hostPort == 0 {
// 			// Auto-assign port - Docker will choose
// 			portBindings[dockerPort] = []docker.PortBinding{{HostIP: "", HostPort: ""}}
// 		} else {
// 			// Use specific host port
// 			portBindings[dockerPort] = []docker.PortBinding{{HostIP: "", HostPort: fmt.Sprintf("%d", hostPort)}}
// 		}
// 	}

// 	// Parse image repository and tag
// 	repository, tag := parseImageString(config.Image)

// 	// Create run options
// 	runOpts := &dockertest.RunOptions{
// 		Name:         config.Name,
// 		Repository:   repository,
// 		Tag:          tag,
// 		NetworkID:    config.Network,
// 		ExposedPorts: exposedPorts,
// 		PortBindings: portBindings,
// 		Mounts:       config.Volumes,
// 		Env:          buildEnvSlice(config.Environment),
// 		User:         config.User,
// 	}

// 	if len(config.EntryPoint) > 0 {
// 		runOpts.Entrypoint = config.EntryPoint
// 	}

// 	if len(config.Cmd) > 0 {
// 		runOpts.Cmd = config.Cmd
// 	}

// 	// Run container with no restart policy
// 	resource, err := cm.Pool.RunWithOptions(runOpts, func(hostConfig *docker.HostConfig) {
// 		hostConfig.RestartPolicy = docker.RestartPolicy{Name: "no"}
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to run container %s: %w", config.Name, err)
// 	}

// 	// Get actual assigned ports
// 	actualPorts := make(map[string]int)
// 	for containerPort := range config.Ports {
// 		hostPort := resource.GetHostPort(fmt.Sprintf("%s/tcp", containerPort))
// 		if hostPort != "" {
// 			// Parse port number from "host:port" format
// 			var port int
// 			if _, err := fmt.Sscanf(hostPort, "%*[^:]:%d", &port); err == nil {
// 				actualPorts[containerPort] = port
// 			}
// 		}
// 	}

// 	container := &Container{
// 		ID:          resource.Container.ID,
// 		Name:        config.Name,
// 		Repository:  config.Image,
// 		NetworkID:   config.Network,
// 		Mounts:      config.Volumes,
// 		Environment: config.Environment,
// 		Resource:    resource,
// 	}

// 	cm.Resources[config.Name] = resource
// 	return container, nil
// }

// StartContainer starts an existing container (if not already started)
func (cm *ContainerManager) StartContainer(container *Container) error {
	// Container is already started when created with RunWithOptions
	return nil
}

// buildEnvSlice converts environment map to slice format expected by Docker
func buildEnvSlice(env map[string]string) []string {
	if env == nil {
		return nil
	}

	envSlice := make([]string, 0, len(env))
	for key, value := range env {
		envSlice = append(envSlice, fmt.Sprintf("%s=%s", key, value))
	}
	return envSlice
}

// parseImageString extracts repository and tag from image string
func parseImageString(image string) (repository, tag string) {
	// Handle cases like "babylonlabs-io/babylon:latest" -> ("babylonlabs-io/babylon", "latest")
	parts := strings.Split(image, ":")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// No tag specified, use latest
	return image, "latest"
}

// sanitizeTestName removes characters that are not valid for Docker network names
func SanitizeTestName(name string) string {
	// Docker network names must be lowercase and can contain a-z, 0-9, _, -, .
	result := ""
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			result += string(r)
		case r >= 'A' && r <= 'Z':
			result += string(r - 'A' + 'a') // convert to lowercase
		case r >= '0' && r <= '9':
			result += string(r)
		case r == '_' || r == '-' || r == '.':
			result += string(r)
		case r == '/' || r == ' ':
			result += "-"
		default:
			// Skip invalid characters
		}
	}

	// Limit length to avoid Docker limitations
	if len(result) > 20 {
		result = result[:20]
	}

	return result
}
