package chain

import (
	sdkmath "cosmossdk.io/math"
	"encoding/json"
	"fmt"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/rollup"
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

func (n *NodeConfig) CommitPubRandListConsumer(consumerId string, fpBtcPk *bbn.BIP340PubKey, startHeight uint64, numPubRand uint64, commitment []byte, sig *bbn.BIP340Signature) {
	if consumerId == "" {
		// Use the chain ID as the consumer
		consumerId = n.chainId
	}
	n.LogActionF("Committing public randomness list for %s", consumerId)

	if consumerId == n.chainId {
		n.CommitPubRandList(fpBtcPk, startHeight, numPubRand, commitment, sig)
		return
	}

	// Get the consumer registration data
	consumer := n.QueryBTCStkConsumerConsumer(consumerId)
	if consumer == nil {
		n.t.Fatalf("Consumer %s not found", consumerId)
	}

	if consumer.ConsumerRegisters == nil || len(consumer.ConsumerRegisters) == 0 {
		n.t.Fatalf("Consumer %s is not registered on %s", consumerId, n.chainId)
	}

	// TODO: Support Cosmos Consumers
	finalityContractAddr := consumer.ConsumerRegisters[0].EthL2FinalityContractAddress
	if finalityContractAddr == "" {
		n.t.Fatalf("Finality contract address for consumer %s is not set", consumerId)
	}

	// Prepare the command to commit the public randomness list
	n.LogActionF("Committing public randomness list to finality contract %s", finalityContractAddr)
	// Prepare the command to commit the public randomness list
	fpPkHex := fpBtcPk.MarshalHex()
	commitPubRandMsg := rollup.CommitPublicRandomnessMsg{
		CommitPublicRandomness: rollup.CommitPublicRandomnessMsgParams{
			FpPubkeyHex: fpPkHex,
			StartHeight: startHeight,
			NumPubRand:  numPubRand,
			Commitment:  commitment,
			Signature:   sig.MustToBTCSig().Serialize(),
		},
	}
	msg, err := json.Marshal(commitPubRandMsg)
	require.NoError(n.t, err)

	cmd := []string{"babylond", "tx", "wasm", "execute", finalityContractAddr, string(msg)}

	// specify used key
	cmd = append(cmd, "--from=val")

	// gas
	cmd = append(cmd, "--gas=500000")

	_, _, err = n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("Successfully committed public randomness list")
}
