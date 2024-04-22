package chain

import (
	"encoding/hex"

	sdkmath "cosmossdk.io/math"
	bbn "github.com/babylonchain/babylon/types"
	bstypes "github.com/babylonchain/babylon/x/btcstaking/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/stretchr/testify/require"
)

func (n *NodeConfig) RegisterConsumerChain(chainId, chainName, chainDesc string) {
	n.LogActionF("registering consumer chain on babylon")

	cmd := []string{
		"babylond", "tx", "btcstkconsumer", "register-chain", chainId, chainName, chainDesc, "--from=val",
	}
	_, _, err := n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully registered consumer chain on babylon")
}

func (n *NodeConfig) CreateConsumerFinalityProvider(babylonPK *secp256k1.PubKey, btcPK *bbn.BIP340PubKey, pop *bstypes.ProofOfPossession, masterPubRand, consumerChainId, moniker, identity, website, securityContract, details string, commission *sdkmath.LegacyDec) {
	n.LogActionF("creating consumer finality provider")

	// get babylon PK hex
	babylonPKBytes, err := babylonPK.Marshal()
	require.NoError(n.t, err)
	babylonPKHex := hex.EncodeToString(babylonPKBytes)
	// get BTC PK hex
	btcPKHex := btcPK.MarshalHex()
	// get pop hex
	popHex, err := pop.ToHexStr()
	require.NoError(n.t, err)

	cmd := []string{
		"babylond", "tx", "btcstaking", "create-finality-provider", babylonPKHex, btcPKHex, popHex, masterPubRand, "--from=val", "--consumer-chain-id", consumerChainId, "--moniker", moniker, "--identity", identity, "--website", website, "--security-contact", securityContract, "--details", details, "--commission-rate", commission.String(),
	}
	_, _, err = n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
	n.LogActionF("successfully created consumer finality provider")
}
