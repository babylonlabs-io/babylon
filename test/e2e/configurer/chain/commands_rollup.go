package chain

import (
	"encoding/json"
	"fmt"

	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"

	"github.com/babylonlabs-io/babylon/v3/types"

	"github.com/babylonlabs-io/babylon/v3/test/e2e/configurer/rollup"
	"github.com/stretchr/testify/require"
)

func (n *NodeConfig) CommitPubRandListRollup(walletAddrOrName, finalityContractAddr string, fpBtcPk *types.BIP340PubKey, startHeight uint64, numPubRand uint64, commitment []byte, sig *types.BIP340Signature) {
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
	cmd = append(cmd, fmt.Sprintf("--from=%s", walletAddrOrName))

	// gas
	cmd = append(cmd, "--gas=500000")

	_, _, err = n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
}

func (n *NodeConfig) AddFinalitySigRollup(
	walletAddrOrName,
	finalityContractAddr string,
	fpBtcPk *types.BIP340PubKey,
	blockHeight uint64,
	pubRand *types.SchnorrPubRand,
	proof cmtcrypto.Proof,
	appHash []byte,
	finalitySig *types.SchnorrEOTSSig,
	overallFlags ...string,
) {
	// Prepare the command to submit the finality signature
	n.LogActionF("Submitting finality signature to finality contract %s", finalityContractAddr)
	// Prepare the command to commit the public randomness list
	fpPkHex := fpBtcPk.MarshalHex()
	submitFinalitySigMsg := rollup.SubmitFinalitySignatureMsg{
		SubmitFinalitySignature: rollup.SubmitFinalitySignatureMsgParams{
			FpPubkeyHex: fpPkHex,
			Height:      blockHeight,
			PubRand:     pubRand.MustMarshal(),
			Proof: rollup.Proof{
				Total:    uint64(proof.Total),
				Index:    uint64(proof.Index),
				LeafHash: proof.LeafHash,
				Aunts:    proof.Aunts,
			},
			Signature: finalitySig.MustMarshal(),
			BlockHash: appHash,
		},
	}
	msg, err := json.Marshal(submitFinalitySigMsg)
	require.NoError(n.t, err)

	cmd := []string{"babylond", "tx", "wasm", "execute", finalityContractAddr, string(msg)}

	// specify used key
	cmd = append(cmd, fmt.Sprintf("--from=%s", walletAddrOrName))

	// gas
	cmd = append(cmd, "--gas=500000")

	_, _, err = n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)
}
