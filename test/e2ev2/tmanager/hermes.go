package tmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/util"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"
)

// HermesRelayer manages Hermes IBC relayer
type HermesRelayer struct {
	Home string

	Endpoint    string
	Container   *Container
	Tm          *TestManager
	ExposedPort int
}

// NewHermesRelayer creates a new Hermes relayer
func NewHermesRelayer(tm *TestManager) *HermesRelayer {
	home := filepath.Join(tm.TempDir, "hermes")
	err := os.MkdirAll(home, 0o755)
	require.NoError(tm.T, err)

	cointanerName := fmt.Sprintf("%s-%s", "hermes", tm.NetworkID()[:4])

	hermesPort, err := tm.PortMgr.AllocatePort()
	require.NoError(tm.T, err)

	return &HermesRelayer{
		Home:        home,
		Container:   NewContainerHermes(cointanerName),
		Tm:          tm,
		ExposedPort: hermesPort,
	}
}

// Start starts the Hermes relayer container
func (hr *HermesRelayer) Start(cA, cB *Chain) {
	dockerResource := hr.RunResource(cA, cB)
	hr.WaitRelayerToStart(dockerResource)
}

func (hr *HermesRelayer) T() *testing.T {
	return hr.Tm.T
}

func (hr *HermesRelayer) RunResource(cA, cB *Chain) *dockertest.Resource {
	hr.T().Log("starting Hermes relayer container...")

	require.GreaterOrEqual(hr.T(), len(cA.Nodes), 1)
	require.GreaterOrEqual(hr.T(), len(cB.Nodes), 1)

	pwd, err := os.Getwd()
	require.NoError(hr.T(), err)

	_, err = util.CopyFile(
		filepath.Join(pwd, "/scripts/", "hermes_bootstrap.sh"),
		filepath.Join(hr.Home, "hermes_bootstrap.sh"),
	)
	require.NoError(hr.T(), err)

	// we are using non validator nodes as validator are constantly sending bls
	// transactions, which makes relayer operations failing
	rnA := cA.Nodes[0]
	rnB := cB.Nodes[0]

	runOpts := &dockertest.RunOptions{
		Name:       hr.Container.Name,
		Repository: hr.Container.Repository,
		Tag:        hr.Container.Tag,
		NetworkID:  hr.Tm.NetworkID(),
		Cmd: []string{
			"start",
		},
		User: "root:root",
		Mounts: []string{
			fmt.Sprintf("%s/:/root/hermes", hr.Home),
		},
		ExposedPorts: hr.ContainerExposedPorts(),
		Env: []string{
			fmt.Sprintf("HERMES_PORT=%d", hr.ExposedPort),
			fmt.Sprintf("BBN_A_E2E_CHAIN_ID=%s", cA.ChainID()),
			fmt.Sprintf("BBN_B_E2E_CHAIN_ID=%s", cB.ChainID()),
			fmt.Sprintf("BBN_A_E2E_VAL_MNEMONIC=%s", rnA.DefaultWallet().Mnemonic),
			fmt.Sprintf("BBN_B_E2E_VAL_MNEMONIC=%s", rnB.DefaultWallet().Mnemonic),
			fmt.Sprintf("BBN_A_E2E_VAL_HOST_GRPC=%s:%d", rnA.Container.Name, rnA.Ports.GRPC),
			fmt.Sprintf("BBN_A_E2E_VAL_HOST_RPC=%s:%d", rnA.Container.Name, rnA.Ports.RPC),
			fmt.Sprintf("BBN_B_E2E_VAL_HOST_GRPC=%s:%d", rnB.Container.Name, rnB.Ports.GRPC),
			fmt.Sprintf("BBN_B_E2E_VAL_HOST_RPC=%s:%d", rnB.Container.Name, rnB.Ports.RPC),
		},
		Entrypoint: []string{
			"sh",
			"-c",
			"chmod +x /root/hermes/hermes_bootstrap.sh && /root/hermes/hermes_bootstrap.sh",
		},
	}

	resource, err := hr.Tm.ContainerManager.Pool.RunWithOptions(runOpts, NoRestart)
	require.NoError(hr.T(), err)

	hr.Tm.ContainerManager.Resources[hr.Container.Name] = resource
	hr.T().Logf("started Hermes relayer container: %s", resource.Container.ID)
	return resource
}

func (hr *HermesRelayer) WaitRelayerToStart(hermesResource *dockertest.Resource) {
	hr.Endpoint = fmt.Sprintf("http://%s/state", hermesResource.GetHostPort(fmt.Sprintf("%d/tcp", hr.ExposedPort)))

	require.Eventually(hr.T(), func() bool {
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			hr.Endpoint,
			nil,
		)
		if err != nil {
			return false
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()

		bz, err := io.ReadAll(resp.Body)
		if err != nil {
			return false
		}

		var respBody map[string]interface{}
		if err := json.Unmarshal(bz, &respBody); err != nil {
			return false
		}

		status, ok := respBody["status"].(string)
		require.True(hr.T(), ok)
		result, ok := respBody["result"].(map[string]interface{})
		require.True(hr.T(), ok)
		chains, ok := result["chains"].([]interface{})
		require.True(hr.T(), ok)

		return status == "success" && len(chains) == 2
	}, 5*time.Minute, time.Second, "hermes relayer not healthy")
}

func (hr *HermesRelayer) ContainerExposedPorts() []string {
	return []string{
		strconv.FormatInt(int64(hr.ExposedPort), 10),
	}
}

// CreateIBCTransferChannel creates a transfer channel between two chains
func (hr *HermesRelayer) CreateIBCTransferChannel(chainA, chainB *Chain) {
	err := hr.CreateIBCChannel(chainA, chainB, "transfer", "transfer", "unordered", "ics20-1", "--b-chain", chainB.ChainID(), "--new-client-connection")
	require.NoError(hr.T(), err)
}

// CreateZoneConciergeChannel creates a consumer channel between two chains using the zoneconcierge port
func (hr *HermesRelayer) CreateZoneConciergeChannel(chainA, chainB *Chain, chainAConnID string) error {
	return hr.CreateIBCChannel(chainA, chainB, "zoneconcierge", "zoneconcierge", "ordered", "zoneconcierge-1", "--a-connection", chainAConnID)
}

// CreateIBCChannel creates an IBC channel between two chains using hermes
func (hr *HermesRelayer) CreateIBCChannel(chainA, chainB *Chain, srcPortID, destPortID, order, version string, otherFlags ...string) error {
	hr.T().Logf("connecting %s and %s chains via IBC: src port %q; dest port %q", chainA.ChainID(), chainB.ChainID(), srcPortID, destPortID)

	cmd := []string{"hermes", "create", "channel",
		"--a-chain", chainA.ChainID(),
		"--a-port", srcPortID, "--b-port", destPortID,
		"--order", order, "--channel-version", version, "--yes"}
	cmd = append(cmd, otherFlags...)

	hr.T().Log(cmd)

	// Execute hermes command in the container
	_, err := hr.ExecHermesCmd(cmd)
	if err != nil {
		return err
	}

	hr.T().Logf("connected %s and %s chains via IBC src port %q; dest port %q", chainA.ChainID(), chainB.ChainID(), srcPortID, destPortID)
	return nil
}

// ExecHermesCmd executes a hermes command in the container
func (hr *HermesRelayer) ExecHermesCmd(cmd []string) (string, error) {
	outBuf, errBuf, err := hr.Tm.ContainerManager.ExecCmd(hr.T(), hr.Container.Name, cmd, "SUCCESS")
	if err != nil {
		return "", fmt.Errorf("failed to exec hermes command: %w", err)
	}

	output := outBuf.String()
	if errOutput := errBuf.String(); errOutput != "" {
		output += "\nstderr: " + errOutput
	}

	return output, nil
}
