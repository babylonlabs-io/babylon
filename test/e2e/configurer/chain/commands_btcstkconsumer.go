package chain

import (
	"fmt"

	sdkmath "cosmossdk.io/math"

	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	bstypes "github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"

	"github.com/stretchr/testify/require"
)

// RegisterConsumerChain registers a Consumer chain
// TODO: Add support for other types of consumer chains
func (n *NodeConfig) RegisterConsumerChain(walletAddrOrName, id, name, description, commission string) {
	n.RegisterRollupConsumerChain(walletAddrOrName, id, name, description, commission, "")
}

// RegisterRollupConsumerChain registers a Rollup (Eth L2) Consumer chain
func (n *NodeConfig) RegisterRollupConsumerChain(walletAddrOrName, id, name, description, commission, finalityContractAddr string) {
	n.LogActionF("Registering consumer chain")
	cmd := []string{
		"babylond", "tx", "btcstkconsumer", "register-consumer", id, name, description, commission, finalityContractAddr,
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
		"--bsn-id", consumerID,
	}

	_, _, err = n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("Successfully created %s finality provider", consumer)
}

func (n *NodeConfig) CreateFinalityProviderV2(walletAddrOrName string, btcPK *bbn.BIP340PubKey, pop *bstypes.ProofOfPossessionBTC, moniker, identity, website, securityContract, details string, commission *sdkmath.LegacyDec, commissionMaxRate, commissionMaxRateChange sdkmath.LegacyDec) {
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
	}

	_, _, err = n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("Successfully created %s finality provider", n.Name)
}

func (n *NodeConfig) CommitPubRandListConsumer(walletAddrOrName, consumerId string, fpBtcPk *bbn.BIP340PubKey, startHeight uint64, numPubRand uint64, commitment []byte, sig *bbn.BIP340Signature) {
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

	if len(consumer.ConsumerRegisters) == 0 {
		n.t.Fatalf("Consumer %s is not registered on %s", consumerId, n.chainId)
	}

	finalityContractAddr := consumer.ConsumerRegisters[0].RollupFinalityContractAddress
	// TODO: Support Cosmos Consumers
	if finalityContractAddr == "" {
		n.t.Fatalf("Finality contract address for consumer %s is not set", consumerId)
	}

	n.CommitPubRandListRollup(walletAddrOrName, finalityContractAddr, fpBtcPk, startHeight, numPubRand, commitment, sig)
	n.LogActionF("Successfully committed public randomness list")
}

func (n *NodeConfig) AddFinalitySigConsumer(
	walletAddrOrName,
	consumerId string,
	fpBTCPK *bbn.BIP340PubKey,
	blockHeight uint64,
	pubRand *bbn.SchnorrPubRand,
	proof cmtcrypto.Proof,
	appHash []byte,
	finalitySig *bbn.SchnorrEOTSSig,
	overallFlags ...string,
) string {
	if consumerId == "" {
		// Use the chain ID as the consumer
		consumerId = n.chainId
	}
	n.LogActionF("Submitting finality signature for %s", consumerId)

	if consumerId == n.chainId {
		n.AddFinalitySig(fpBTCPK, blockHeight, pubRand, proof, appHash, finalitySig, overallFlags...)
		return ""
	}

	// Get the consumer registration data
	consumer := n.QueryBTCStkConsumerConsumer(consumerId)
	if consumer == nil {
		n.t.Fatalf("Consumer %s not found", consumerId)
	}

	if len(consumer.ConsumerRegisters) == 0 {
		n.t.Fatalf("Consumer %s is not registered on %s", consumerId, n.chainId)
	}

	finalityContractAddr := consumer.ConsumerRegisters[0].RollupFinalityContractAddress
	// TODO: Support Cosmos Consumers
	if finalityContractAddr == "" {
		n.t.Fatalf("Finality contract address for consumer %s is not set", consumerId)
	}

	txHash := n.AddFinalitySigRollup(walletAddrOrName, finalityContractAddr, fpBTCPK, blockHeight, pubRand, proof, appHash, finalitySig, overallFlags...)
	n.LogActionF("Successfully added finality signature")

	return txHash
}
