package containers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

const (
	hermesContainerName        = "hermes-relayer"
	cosmosRelayerContainerName = "rly-relayer"
	// The maximum number of times debug logs are printed to console
	// per CLI command.
	maxDebugLogsPerCommand = 3
	BabylonHomePath        = "/home/babylon/babylondata"
	FlagHome               = "--home=" + BabylonHomePath
)

var errRegex = regexp.MustCompile(`(E|e)rror`)

// Manager is a wrapper around all Docker instances, and the Docker API.
// It provides utilities to run and interact with all Docker containers used within e2e testing.
type Manager struct {
	ImageConfig
	pool              *dockertest.Pool
	network           *dockertest.Network
	resources         map[string]*dockertest.Resource
	isDebugLogEnabled bool
	identifier        string
}

// NewManager creates a new Manager instance and initializes
// all Docker specific utilities. Returns an error if initialization fails.
func NewManager(identifier string, isDebugLogEnabled bool, isCosmosRelayer, isUpgrade bool) (m *Manager, err error) {
	m = &Manager{
		ImageConfig:       NewImageConfig(isCosmosRelayer, isUpgrade),
		resources:         make(map[string]*dockertest.Resource),
		isDebugLogEnabled: isDebugLogEnabled,
		identifier:        identifier,
	}
	m.pool, err = dockertest.NewPool("")
	if err != nil {
		return nil, err
	}
	m.network, err = m.pool.CreateNetwork(m.NetworkName())
	if err != nil {
		return nil, err
	}
	return m, nil
}

// ExecTxCmd Runs ExecTxCmdWithSuccessString searching for `code: 0`
func (m *Manager) ExecTxCmd(t *testing.T, chainId string, nodeName string, command []string) (
	outBuf, errBuf bytes.Buffer,
	err error,
) {
	return m.ExecTxCmdWithSuccessString(t, chainId, nodeName, command, "code: 0")
}

// ExecTxCmdWithSuccessString Runs ExecCmd, with flags for txs added.
// namely adding flags `--chain-id={chain-id} -b=block --yes --keyring-backend=test "--log_format=json"`,
// and searching for `successStr`
func (m *Manager) ExecTxCmdWithSuccessString(t *testing.T, chainId string, containerName string, command []string, successStr string) (bytes.Buffer, bytes.Buffer, error) {
	additionalArgs := []string{fmt.Sprintf("--chain-id=%s", chainId), "--gas-prices=1ubbn", "-b=sync", "--yes", "--keyring-backend=test", "--log_format=json", "--home=/home/babylon/babylondata"}

	cmd := command
	cmd = append(cmd, additionalArgs...)

	return m.ExecCmd(t, containerName, cmd, successStr)
}

// ExecHermesCmd executes command on the hermes relaer container.
func (m *Manager) ExecHermesCmd(t *testing.T, command []string, success string) (bytes.Buffer, bytes.Buffer, error) {
	return m.ExecCmd(t, m.HermesContainerName(), command, success)
}

// ExecCmd executes command by running it on the node container (specified by containerName)
// success is the output of the command that needs to be observed for the command to be deemed successful.
// It is found by checking if stdout or stderr contains the success string anywhere within it.
// returns container std out, container std err, and error if any.
// An error is returned if the command fails to execute or if the success string is not found in the output.
func (m *Manager) ExecCmd(t *testing.T, fullContainerName string, command []string, success string) (bytes.Buffer, bytes.Buffer, error) {
	if _, ok := m.resources[fullContainerName]; !ok {
		return bytes.Buffer{}, bytes.Buffer{}, fmt.Errorf("no resource %s found", fullContainerName)
	}
	containerId := m.resources[fullContainerName].Container.ID

	var (
		outBuf bytes.Buffer
		errBuf bytes.Buffer
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if m.isDebugLogEnabled {
		t.Logf("\n\nRunning: \"%s\", success condition is \"%s\"", command, success)
	}
	maxDebugLogTriesLeft := maxDebugLogsPerCommand

	// We use the `require.Eventually` function because it is only allowed to do one transaction per block without
	// sequence numbers. For simplicity, we avoid keeping track of the sequence number and just use the `require.Eventually`.
	require.Eventually(
		t,
		func() bool {
			exec, err := m.pool.Client.CreateExec(docker.CreateExecOptions{
				Context:      ctx,
				AttachStdout: true,
				AttachStderr: true,
				Container:    containerId,
				User:         "root",
				Cmd:          command,
			})
			require.NoError(t, err)

			err = m.pool.Client.StartExec(exec.ID, docker.StartExecOptions{
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

			if (errRegex.MatchString(errBufString) || m.isDebugLogEnabled) && maxDebugLogTriesLeft > 0 {
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

// RunHermesResource runs a Hermes container. Returns the container resource and error if any.
// the name of the hermes container is "<chain A id>-<chain B id>-relayer"
func (m *Manager) RunHermesResource(chainAID, osmoARelayerNodeName, osmoAValMnemonic, chainBID, osmoBRelayerNodeName, osmoBValMnemonic string, hermesCfgPath string) (*dockertest.Resource, error) {
	hermesResource, err := m.pool.RunWithOptions(
		&dockertest.RunOptions{
			Name:       m.HermesContainerName(),
			Repository: m.RelayerRepository,
			Tag:        m.RelayerTag,
			NetworkID:  m.network.Network.ID,
			Cmd: []string{
				"start",
			},
			User: "root:root",
			Mounts: []string{
				fmt.Sprintf("%s/:/root/hermes", hermesCfgPath),
			},
			ExposedPorts: []string{
				"3031",
			},
			PortBindings: map[docker.Port][]docker.PortBinding{
				"3031/tcp": {{HostIP: "", HostPort: "3031"}},
			},
			Env: []string{
				fmt.Sprintf("BBN_A_E2E_CHAIN_ID=%s", chainAID),
				fmt.Sprintf("BBN_B_E2E_CHAIN_ID=%s", chainBID),
				fmt.Sprintf("BBN_A_E2E_VAL_MNEMONIC=%s", osmoAValMnemonic),
				fmt.Sprintf("BBN_B_E2E_VAL_MNEMONIC=%s", osmoBValMnemonic),
				fmt.Sprintf("BBN_A_E2E_VAL_HOST=%s", osmoARelayerNodeName),
				fmt.Sprintf("BBN_B_E2E_VAL_HOST=%s", osmoBRelayerNodeName),
			},
			Entrypoint: []string{
				"sh",
				"-c",
				"chmod +x /root/hermes/hermes_bootstrap.sh && /root/hermes/hermes_bootstrap.sh",
			},
		},
		noRestart,
	)
	if err != nil {
		return nil, err
	}
	m.resources[m.HermesContainerName()] = hermesResource
	return hermesResource, nil
}

// RunRlyResource runs a Cosmos relayer container. Returns the container resource and error if any.
// the name of the cosmos container is "<chain A id>-<chain B id>-relayer"
func (m *Manager) RunRlyResource(chainAID, osmoARelayerNodeName, osmoAValMnemonic, chainAIbcPort, chainBID, osmoBRelayerNodeName, osmoBValMnemonic, chainBIbcPort string, rlyCfgPath string) (*dockertest.Resource, error) {
	rlyResource, err := m.pool.RunWithOptions(
		&dockertest.RunOptions{
			Name:       m.CosmosRlyrContainerName(),
			Repository: m.RelayerRepository,
			Tag:        m.RelayerTag,
			NetworkID:  m.network.Network.ID,
			Cmd: []string{
				"start",
			},
			User: "root:root",
			Mounts: []string{
				fmt.Sprintf("%s/:/root/rly", rlyCfgPath),
			},
			Env: []string{
				fmt.Sprintf("BBN_A_E2E_CHAIN_ID=%s", chainAID),
				fmt.Sprintf("BBN_B_E2E_CHAIN_ID=%s", chainBID),
				fmt.Sprintf("BBN_A_E2E_VAL_MNEMONIC=%s", osmoAValMnemonic),
				fmt.Sprintf("BBN_B_E2E_VAL_MNEMONIC=%s", osmoBValMnemonic),
				fmt.Sprintf("BBN_A_E2E_VAL_HOST=%s", osmoARelayerNodeName),
				fmt.Sprintf("BBN_B_E2E_VAL_HOST=%s", osmoBRelayerNodeName),
				fmt.Sprintf("CHAIN_A_IBC_PORT=%s", chainAIbcPort),
				fmt.Sprintf("CHAIN_B_IBC_PORT=%s", chainBIbcPort),
			},
			Entrypoint: []string{
				"sh",
				"-c",
				"chmod +x /root/rly/rly_bootstrap.sh && /root/rly/rly_bootstrap.sh",
			},
		},
		noRestart,
	)
	if err != nil {
		return nil, err
	}
	m.resources[m.CosmosRlyrContainerName()] = rlyResource
	return rlyResource, nil
}

// RunNodeResource runs a node container. Assigns containerName to the container.
// Mounts the container on valConfigDir volume on the running host. Returns the container resource and error if any.
func (m *Manager) RunNodeResource(chainId string, containerName, valCondifDir string) (*dockertest.Resource, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	runOpts := &dockertest.RunOptions{
		Name:       containerName,
		Repository: m.CurrentRepository,
		Tag:        m.CurrentTag,
		NetworkID:  m.network.Network.ID,
		User:       "root:root",
		Entrypoint: []string{
			"sh",
			"-c",
			// Use the following for debugging purposes:
			// TODO: Parameterise the log level
			// "export BABYLON_BLS_PASSWORD=password && babylond start " + FlagHome + " --log_level trace --trace",
			"export BABYLON_BLS_PASSWORD=password && babylond start " + FlagHome,
			// Alternative option: use --no-bls-password flag
			// "babylond start " + FlagHome + " --no-bls-password",
		},
		ExposedPorts: []string{"26656", "26657", "1317", "9090"},
		Mounts: []string{
			fmt.Sprintf("%s/:%s", valCondifDir, BabylonHomePath),
			fmt.Sprintf("%s/bytecode:/bytecode", pwd),
			fmt.Sprintf("%s/govProps:/govProps", pwd),
		},
	}

	resource, err := m.pool.RunWithOptions(runOpts, noRestart)
	if err != nil {
		return nil, err
	}

	m.resources[containerName] = resource

	return resource, nil
}

// PurgeResource purges the container resource and returns an error if any.
func (m *Manager) PurgeResource(resource *dockertest.Resource) error {
	return m.pool.Purge(resource)
}

// GetNodeResource returns the node resource for containerName.
func (m *Manager) GetNodeResource(containerName string) (*dockertest.Resource, error) {
	resource, exists := m.resources[containerName]
	if !exists {
		return nil, fmt.Errorf("node resource not found: container name: %s", containerName)
	}
	return resource, nil
}

// GetHostPort returns the port-forwarding address of the running host
// necessary to connect to the portId exposed inside the container.
// The container is determined by containerName.
// Returns the host-port or error if any.
func (m *Manager) GetHostPort(nodeName string, portId string) (string, error) {
	resource, err := m.GetNodeResource(nodeName)
	if err != nil {
		return "", err
	}
	return resource.GetHostPort(portId), nil
}

// RemoveNodeResource removes a node container specified by containerName.
// Returns error if any.
func (m *Manager) RemoveNodeResource(containerName string) error {
	resource, err := m.GetNodeResource(containerName)
	if err != nil {
		return err
	}
	var opts docker.RemoveContainerOptions
	opts.ID = resource.Container.ID
	opts.Force = true
	if err := m.pool.Client.RemoveContainer(opts); err != nil {
		return err
	}
	delete(m.resources, containerName)
	return nil
}

// ClearResources removes all outstanding Docker resources created by the Manager.
func (m *Manager) ClearResources() (e error) {
	g := new(errgroup.Group)
	for _, resource := range m.resources {
		resource := resource
		g.Go(func() error {
			return m.pool.Purge(resource)
		})
	}

	// TODO: fix error to delete wasm
	// unlinkat /tmp/bbn-e2e-testnet-2820217771/bbn-test-a/bbn-test-a-node-babylon-default-a-2/ibc_08-wasm/state/wasm: permission denied
	err := g.Wait()
	if err != nil {
		fmt.Printf("error to clear resources %s", err.Error())
	}

	return errors.Join(err, m.pool.RemoveNetwork(m.network))
}

func noRestart(config *docker.HostConfig) {
	// in this case we don't want the nodes to restart on failure
	config.RestartPolicy = docker.RestartPolicy{
		Name: "no",
	}
}

// RunChainInitResource runs a chain init container to initialize genesis and configs for a chain with chainId.
// The chain is to be configured with chainVotingPeriod and validators deserialized from validatorConfigBytes.
// The genesis and configs are to be mounted on the init container as volume on mountDir path.
// Returns the container resource and error if any. This method does not Purge the container. The caller
// must deal with removing the resource.
func (m *Manager) RunChainInitResource(
	chainId string,
	chainVotingPeriod, chainExpeditedVotingPeriod int,
	validatorInitConfigBytesHexEncoded string,
	mountDir string,
	forkHeight int,
	btcHeaders string,
) (*dockertest.Resource, error) {
	votingPeriodDuration := time.Duration(chainVotingPeriod * 1000000000)
	expeditedVotingPeriodDuration := time.Duration(chainExpeditedVotingPeriod * 1000000000)

	// Note: any change that needs to take effect in older releases, lets say
	// that it is needed to update the config of some node in the TGE chain
	// for software upgrade testing, it is needed to also update the version
	// from that babylon node, probably a new tag will need to be pushed in
	// older releases branches increasing the minor patch.
	initResource, err := m.pool.RunWithOptions(
		&dockertest.RunOptions{
			Name:       chainId,
			Repository: InitChainContainerE2E,
			NetworkID:  m.network.Network.ID,
			Cmd: []string{
				fmt.Sprintf("--data-dir=%s", mountDir),
				fmt.Sprintf("--chain-id=%s", chainId),
				fmt.Sprintf("--config=%s", validatorInitConfigBytesHexEncoded),
				fmt.Sprintf("--voting-period=%v", votingPeriodDuration),
				fmt.Sprintf("--expedited-voting-period=%v", expeditedVotingPeriodDuration),
				fmt.Sprintf("--fork-height=%v", forkHeight),
				fmt.Sprintf("--btc-headers=%s", btcHeaders),
			},
			User: "root:root",
			Mounts: []string{
				fmt.Sprintf("%s:%s", mountDir, mountDir),
			},
		},
		noRestart,
	)
	if err != nil {
		return nil, err
	}
	return initResource, nil
}

// NetworkName returns the network name concatenated with the identifier name
func (m *Manager) NetworkName() string {
	return fmt.Sprintf("bbn-testnet-%s", m.identifier)
}

// HermesContainerName returns the hermes container name concatenated with the
// identifier
func (m *Manager) HermesContainerName() string {
	return fmt.Sprintf("%s-%s", hermesContainerName, m.identifier)
}

// CosmosRlyrContainerName returns the cosmos relayer container name
// concatenated with the identifier
func (m *Manager) CosmosRlyrContainerName() string {
	return fmt.Sprintf("%s-%s", cosmosRelayerContainerName, m.identifier)
}

// WithCurrentTag sets the current tag of the babylon image in the manager
// This function is useful when we want to test an upgrade from a specific tag
// overriding the default BabylonContainerTagBeforeUpgrade value
func (m *Manager) WithCurrentTag(tag string) {
	m.ImageConfig.CurrentTag = tag
}
