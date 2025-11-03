package tmanager

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

const (
	// Images that do not have specified tag, latest will be used by default.
	// name of babylon image produced by running `make build-docker`
	BabylonContainerName = "babylonlabs-io/babylond"
	// name of babylon image before the upgrade
	BabylonContainerNameBeforeUpgrade = "babylonlabs/babylond"
	BabylonContainerTagBeforeUpgrade  = "v4.0.0-rc.1"

	HermesRelayerRepository = "informalsystems/hermes"
	HermesRelayerTag        = "1.13.1"
)

var (
	errRegex               = regexp.MustCompile(`(E|e)rror`)
	maxDebugLogsPerCommand = 3
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

// NewContainerOldBbnNode create an older binary version of a bbn node which is used before upgrade
func NewContainerOldBbnNode(containerName, tag string) *Container {
	cTag := BabylonContainerTagBeforeUpgrade
	if tag != "" {
		cTag = tag
	}

	return &Container{
		Name:       containerName,
		Repository: BabylonContainerNameBeforeUpgrade,
		Tag:        cTag,
	}
}

func NewContainerHermes(containerName string) *Container {
	return &Container{
		Name:       containerName,
		Repository: HermesRelayerRepository,
		Tag:        HermesRelayerTag,
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

// StartContainer starts an existing container (if not already started)
func (cm *ContainerManager) StartContainer(container *Container) error {
	// Container is already started when created with RunWithOptions
	return nil
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

// ExecCmd executes a command in a container
func (cm *ContainerManager) ExecCmd(t *testing.T, fullContainerName string, command []string, success string) (bytes.Buffer, bytes.Buffer, error) {
	cm.Mutex.RLock()
	resource, ok := cm.Resources[fullContainerName]
	cm.Mutex.RUnlock()

	if !ok {
		return bytes.Buffer{}, bytes.Buffer{}, fmt.Errorf("no resource %s found", fullContainerName)
	}
	containerId := resource.Container.ID

	var (
		outBuf bytes.Buffer
		errBuf bytes.Buffer
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	t.Logf("\n\nRunning: \"%s\", success condition is \"%s\"", command, success)
	maxDebugLogTriesLeft := maxDebugLogsPerCommand

	// We use the `require.Eventually` function because it is only allowed to do one transaction per block without
	// sequence numbers. For simplicity, we avoid keeping track of the sequence number and just use the `require.Eventually`.
	require.Eventually(
		t,
		func() bool {
			exec, err := cm.Pool.Client.CreateExec(docker.CreateExecOptions{
				Context:      ctx,
				AttachStdout: true,
				AttachStderr: true,
				Container:    containerId,
				User:         "root",
				Cmd:          command,
			})
			require.NoError(t, err)

			err = cm.Pool.Client.StartExec(exec.ID, docker.StartExecOptions{
				Context:      ctx,
				Detach:       false,
				OutputStream: &outBuf,
				ErrorStream:  &errBuf,
			})
			if err != nil {
				return false
			}

			errBufString := errBuf.String()
			// Note that this does not match all errors.
			// This only works if CLI outputs "Error" or "error"
			// to stderr.
			fmt.Printf("\n Debug: errOut %s", errBufString)
			fmt.Printf("\n Debug: command %+v\noutput %s", command, outBuf.String())

			if (errRegex.MatchString(errBufString)) && maxDebugLogTriesLeft > 0 {
				t.Log("\nstderr:")
				t.Log(errBufString)

				t.Log("\nstdout:")
				t.Log(outBuf.String())
				// N.B: We should not be returning false here
				// because some applications such as Hermes might log
				// "error" to stderr when they function correctly,
				// causing test flakiness. This log is needed only for
				// debugging purposes.
				maxDebugLogTriesLeft--
			}

			if success != "" {
				return strings.Contains(outBuf.String(), success) || strings.Contains(errBufString, success)
			}

			return true
		},
		2*time.Minute,
		50*time.Millisecond,
		"tx returned a non-zero code",
	)

	return outBuf, errBuf, nil
}
