package chain

import (
	"encoding/json"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/rollup"
	"github.com/babylonlabs-io/babylon/v4/types"
	"github.com/stretchr/testify/require"
)

func (n *NodeConfig) CommitPubRandListRollup(finalityContractAddr string, fpBtcPk *types.BIP340PubKey, startHeight uint64, numPubRand uint64, commitment []byte, sig *types.BIP340Signature) {
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
}
