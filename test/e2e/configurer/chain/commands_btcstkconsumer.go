package chain

import (
	sdkmath "cosmossdk.io/math"
	"fmt"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/stretchr/testify/require"
	"strconv"
)

// RegisterConsumerChain registers a Consumer chain
// TODO: Add support for other types of consumer chains
func (n *NodeConfig) RegisterConsumerChain(walletAddrOrName, id, name, description string, maxMultiStaked int) {
	n.RegisterRollupConsumerChain(walletAddrOrName, id, name, description, "", maxMultiStaked)
}

// RegisterRollupConsumerChain registers a Rollup (Eth L2) Consumer chain
func (n *NodeConfig) RegisterRollupConsumerChain(walletAddrOrName, id, name, description, finalityContractAddr string, maxMultiStaked int) {
	n.LogActionF("Registering consumer chain")
	cmd := []string{
		"babylond", "tx", "btcstkconsumer", "register-consumer", id, name, description, strconv.Itoa(maxMultiStaked), finalityContractAddr,
		fmt.Sprintf("--from=%s", walletAddrOrName),
	}
	_, _, err := n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully registered consumer chain")
}

func (n *NodeConfig) CreateConsumerFinalityProvider(walletAddrOrName string, consumerID string, btcPK *bbn.BIP340PubKey, pop *bstypes.ProofOfPossessionBTC, moniker, identity, website, securityContract, details string, commission *sdkmath.LegacyDec, commissionMaxRate, commissionMaxRateChange sdkmath.LegacyDec) {
	// Just for logs
	consumer := consumerID
	if consumer == "" {
		// Use the chain ID as the consumer
		consumer = n.chainId
	}
	n.LogActionF("Creating %s finality provider", consumer)

	// get BTC PK hex
	btcPKHex := btcPK.MarshalHex()
	// get pop hex
	popHex, err := pop.ToHexStr()
	require.NoError(n.t, err)

	cmd := []string{
		"babylond", "tx", "btcstaking", "create-finality-provider", btcPKHex, popHex,
		fmt.Sprintf("--from=%s", walletAddrOrName), "--moniker", moniker, "--identity", identity, "--website", website,
		"--security-contact", securityContract, "--details", details, "--commission-rate", commission.String(),
		"--commission-max-rate", commissionMaxRate.String(), "--commission-max-change-rate", commissionMaxRateChange.String(),
		"--consumer-id", consumerID,
	}

	_, _, err = n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("Successfully created %s finality provider", consumer)
}
