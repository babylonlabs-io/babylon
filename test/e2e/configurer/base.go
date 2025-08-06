package configurer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/chain"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/containers"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/initialization"
	"github.com/babylonlabs-io/babylon/v3/test/e2e/util"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
)

// baseConfigurer is the base implementation for the
// other 2 types of configurers. It is not meant to be used
// on its own. Instead, it is meant to be embedded
// by composition into more concrete configurers.
type baseConfigurer struct {
	chainConfigs     []*chain.Config
	containerManager *containers.Manager
	setupTests       setupFn
	syncUntilHeight  int64 // the height until which to wait for validators to sync when first started.
	t                *testing.T
}

// defaultSyncUntilHeight arbitrary small height to make sure the chain is making progress.
const defaultSyncUntilHeight = 3

func (bc *baseConfigurer) ClearResources() error {
	bc.t.Log("tearing down e2e integration test suite...")

	if err := bc.containerManager.ClearResources(); err != nil {
		return err
	}

	g := new(errgroup.Group)
	for _, chainConfig := range bc.chainConfigs {
		chainConfig := chainConfig
		g.Go(func() error {
			return os.RemoveAll(chainConfig.DataDir)
		})
	}
	return g.Wait()
}

func (bc *baseConfigurer) GetChainConfig(chainIndex int) *chain.Config {
	return bc.chainConfigs[chainIndex]
}

func (bc *baseConfigurer) RunValidators() error {
	for _, chainConfig := range bc.chainConfigs {
		if err := bc.runValidators(chainConfig); err != nil {
			return err
		}
	}
	return nil
}

func (bc *baseConfigurer) runValidators(chainConfig *chain.Config) error {
	bc.t.Logf("starting %s validator containers...", chainConfig.Id)
	for _, node := range chainConfig.NodeConfigs {
		if err := node.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (bc *baseConfigurer) RunHermesRelayerIBC() error {
	// Run a relayer between every possible pair of chains.
	for i := 0; i < len(bc.chainConfigs); i++ {
		for j := i + 1; j < len(bc.chainConfigs); j++ {
			if err := bc.runHermesIBCRelayer(bc.chainConfigs[i], bc.chainConfigs[j]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (bc *baseConfigurer) RunCosmosRelayerIBC() error {
	// Run a relayer between every possible pair of chains.
	for i := 0; i < len(bc.chainConfigs); i++ {
		for j := i + 1; j < len(bc.chainConfigs); j++ {
			if err := bc.runCosmosIBCRelayer(bc.chainConfigs[i], bc.chainConfigs[j]); err != nil {
				return err
			}
		}
	}
	// Launches a relayer between chain A (babylond) and chain B (wasmd)
	return nil
}

func (bc *baseConfigurer) RunIBCTransferChannel() error {
	// Run a relayer between every possible pair of chains.
	for i := 0; i < len(bc.chainConfigs); i++ {
		for j := i + 1; j < len(bc.chainConfigs); j++ {
			if err := bc.runHermesIBCRelayer(bc.chainConfigs[i], bc.chainConfigs[j]); err != nil {
				return err
			}
			if err := bc.createIBCTransferChannel(bc.chainConfigs[i], bc.chainConfigs[j]); err != nil {
				return err
			}
		}
	}
	return nil
}

// OpenZoneConciergeChannel opens a zoneconcierge channel between all pairs of chains.
// This function assumes relayers are already running
func (bc *baseConfigurer) OpenZoneConciergeChannel(chainA, chainB *chain.Config, chainAConnID string) error {
	return bc.createZoneConciergeChannel(chainA, chainB, chainAConnID)
}

// CompleteIBCChannelHandshake completes the channel handshake in cases when ChanOpenInit was initiated
// by some transaction that was previously executed on the chain. For example,
// ICA MsgRegisterInterchainAccount will perform ChanOpenInit during its execution.
func (bc *baseConfigurer) CompleteIBCChannelHandshake(
	srcChain, dstChain,
	srcConnection, dstConnection,
	srcPort, dstPort,
	srcChannel, dstChannel string,
) error {
	bc.t.Logf("completing IBC channel handshake between: (%s, %s, %s, %s) and (%s, %s, %s, %s)",
		srcChain, srcConnection, srcPort, srcChannel,
		dstChain, dstConnection, dstPort, dstChannel)

	cmd := []string{
		"hermes",
		"--json",
		"tx",
		"chan-open-try",
		"--dst-chain", dstChain,
		"--src-chain", srcChain,
		"--dst-connection", dstConnection,
		"--dst-port", dstPort,
		"--src-port", srcPort,
		"--src-channel", srcChannel,
	}

	bc.t.Log(cmd)
	_, _, err := bc.containerManager.ExecHermesCmd(bc.t, cmd, "success")
	if err != nil {
		return err
	}

	cmd = []string{
		"hermes",
		"--json",
		"tx",
		"chan-open-ack",
		"--dst-chain", srcChain,
		"--src-chain", dstChain,
		"--dst-connection", srcConnection,
		"--dst-port", srcPort,
		"--src-port", dstPort,
		"--dst-channel", srcChannel,
		"--src-channel", dstChannel,
	}

	bc.t.Log(cmd)
	_, _, err = bc.containerManager.ExecHermesCmd(bc.t, cmd, "")
	if err != nil {
		return err
	}
	cmd = []string{
		"hermes",
		"--json",
		"tx",
		"chan-open-confirm",
		"--dst-chain", dstChain,
		"--src-chain", srcChain,
		"--dst-connection", dstConnection,
		"--dst-port", dstPort,
		"--src-port", srcPort,
		"--dst-channel", dstChannel,
		"--src-channel", srcChannel,
	}

	bc.t.Log(cmd)
	_, _, err = bc.containerManager.ExecHermesCmd(bc.t, cmd, "")
	if err != nil {
		return err
	}

	bc.t.Logf("IBC channel handshake completed between: (%s, %s, %s, %s) and (%s, %s, %s, %s)",
		srcChain, srcConnection, srcPort, srcChannel,
		dstChain, dstConnection, dstPort, dstChannel)

	return nil
}

func (bc *baseConfigurer) runHermesIBCRelayer(chainConfigA *chain.Config, chainConfigB *chain.Config) error {
	bc.t.Log("starting Hermes relayer container...")

	tmpDir, err := os.MkdirTemp("", "bbn-e2e-testnet-hermes-")
	if err != nil {
		return err
	}

	hermesCfgPath := path.Join(tmpDir, "hermes")

	if err := os.MkdirAll(hermesCfgPath, 0o755); err != nil {
		return err
	}

	_, err = util.CopyFile(
		filepath.Join("./scripts/", "hermes_bootstrap.sh"),
		filepath.Join(hermesCfgPath, "hermes_bootstrap.sh"),
	)
	if err != nil {
		return err
	}

	// we are using non validator nodes as validator are constantly sending bls
	// transactions, which makes relayer operations failing
	relayerNodeA := chainConfigA.NodeConfigs[2]
	relayerNodeB := chainConfigB.NodeConfigs[2]

	hermesResource, err := bc.containerManager.RunHermesResource(
		chainConfigA.Id,
		relayerNodeA.Name,
		relayerNodeA.Mnemonic,
		chainConfigB.Id,
		relayerNodeB.Name,
		relayerNodeB.Mnemonic,
		hermesCfgPath)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("http://%s/state", hermesResource.GetHostPort("3031/tcp"))

	require.Eventually(bc.t, func() bool {
		req, err := http.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			endpoint,
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
		require.True(bc.t, ok)
		result, ok := respBody["result"].(map[string]interface{})
		require.True(bc.t, ok)

		chains, ok := result["chains"].([]interface{})
		require.True(bc.t, ok)

		return status == "success" && len(chains) == 2
	},
		5*time.Minute,
		time.Second,
		"hermes relayer not healthy")

	bc.t.Logf("started Hermes relayer container: %s", hermesResource.Container.ID)

	// XXX: Give time to both networks to start, otherwise we might see gRPC
	// transport errors.
	time.Sleep(3 * time.Second)

	return nil
}

func (bc *baseConfigurer) runCosmosIBCRelayer(chainConfigA *chain.Config, chainConfigB *chain.Config) error {
	bc.t.Log("Starting Cosmos relayer container...")

	tmpDir, err := os.MkdirTemp("", "bbn-e2e-testnet-cosmos-")
	if err != nil {
		return err
	}

	rlyCfgPath := path.Join(tmpDir, "rly")

	if err := os.MkdirAll(rlyCfgPath, 0o755); err != nil {
		return err
	}

	_, err = util.CopyFile(
		filepath.Join("./scripts/", "rly_bootstrap.sh"),
		filepath.Join(rlyCfgPath, "rly_bootstrap.sh"),
	)
	if err != nil {
		return err
	}

	// we are using non validator nodes as validator are constantly sending bls
	// transactions, which makes relayer operations failing
	relayerNodeA := chainConfigA.NodeConfigs[2]
	relayerNodeB := chainConfigB.NodeConfigs[2]

	rlyResource, err := bc.containerManager.RunRlyResource(
		chainConfigA.Id,
		relayerNodeA.Name,
		relayerNodeA.Mnemonic,
		chainConfigA.IBCConfig.PortID,
		chainConfigB.Id,
		relayerNodeB.Name,
		relayerNodeB.Mnemonic,
		chainConfigB.IBCConfig.PortID,
		rlyCfgPath)
	if err != nil {
		return err
	}

	// Wait for the relayer to connect to the chains
	bc.t.Logf("waiting for Cosmos relayer setup...")
	time.Sleep(30 * time.Second)

	bc.t.Logf("started Cosmos relayer container: %s", rlyResource.Container.ID)

	return nil
}

func (bc *baseConfigurer) createIBCTransferChannel(chainA *chain.Config, chainB *chain.Config) error {
	return bc.createIBCChannel(chainA, chainB, "transfer", "transfer", "unordered", "ics20-1", "--new-client-connection")
}

// createZoneConciergeChannel creates a consumer channel between two chains using the zoneconcierge port
func (bc *baseConfigurer) createZoneConciergeChannel(chainA *chain.Config, chainB *chain.Config, chainAConnID string) error {
	return bc.createIBCChannel(chainA, chainB, "zoneconcierge", "zoneconcierge", "ordered", "zoneconcierge-1", "--a-connection", chainAConnID)
}

func (bc *baseConfigurer) createIBCChannel(chainA *chain.Config, chainB *chain.Config, srcPortID, destPortID, order, version string, otherFlags ...string) error {
	bc.t.Logf("connecting %s and %s chains via IBC: src port %q; dest port %q", chainA.ChainMeta.Id, chainB.ChainMeta.Id, srcPortID, destPortID)
	cmd := []string{"hermes", "create", "channel",
		"--a-chain", chainA.ChainMeta.Id, "--b-chain", chainB.ChainMeta.Id,
		"--a-port", srcPortID, "--b-port", destPortID,
		"--order", order, "--channel-version", version, "--yes"}
	cmd = append(cmd, otherFlags...)
	bc.t.Log(cmd)
	_, _, err := bc.containerManager.ExecHermesCmd(bc.t, cmd, "SUCCESS")
	if err != nil {
		return err
	}
	bc.t.Logf("connected %s and %s chains via IBC src port %q; dest port %q", chainA.ChainMeta.Id, chainB.ChainMeta.Id, srcPortID, destPortID)
	return nil
}

func (bc *baseConfigurer) initializeChainConfigFromInitChain(initializedChain *initialization.Chain, chainConfig *chain.Config) {
	chainConfig.ChainMeta = initializedChain.ChainMeta
	chainConfig.NodeConfigs = make([]*chain.NodeConfig, 0, len(initializedChain.Nodes))
	setupTime := time.Now()
	for i, validator := range initializedChain.Nodes {
		conf := chain.NewNodeConfig(bc.t, validator, chainConfig.ValidatorInitConfigs[i], chainConfig.Id, bc.containerManager).WithSetupTime(setupTime)
		chainConfig.NodeConfigs = append(chainConfig.NodeConfigs, conf)
	}
}
