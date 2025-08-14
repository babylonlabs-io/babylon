package chain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	cmtcrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"

	"github.com/babylonlabs-io/babylon/v4/types"

	"github.com/babylonlabs-io/babylon/v4/test/e2e/configurer/rollup"
	"github.com/babylonlabs-io/babylon/v4/test/e2e/initialization"
	"github.com/stretchr/testify/require"
)

const (
	finalityContractPath = "/bytecode/finality.wasm"
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
) string {
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

	outBuf, _, err := n.containerManager.ExecTxCmd(n.t, n.chainId, n.Name, cmd)
	require.NoError(n.t, err)

	return GetTxHashFromOutput(outBuf.String())
}

func (n *NodeConfig) CreateFinalityContract(bsnId string) (contractAddr string) {
	wasmContractId := int(n.QueryLatestWasmCodeID())
	n.StoreWasmCode(finalityContractPath, n.WalletName)

	n.WaitForNextBlock()

	require.Eventually(n.t, func() bool {
		newLatestWasmId := int(n.QueryLatestWasmCodeID())
		if wasmContractId+1 > newLatestWasmId {
			return false
		}
		wasmContractId = newLatestWasmId
		return true
	}, time.Second*6, time.Millisecond*100)

	// Instantiate the finality gadget contract with node as admin
	n.InstantiateWasmContract(
		strconv.Itoa(wasmContractId),
		`{
			"admin": "`+n.PublicAddress+`",
			"bsn_id": "`+bsnId+`"
		}`,
		initialization.ValidatorWalletName,
	)

	var (
		contracts []string
		err       error
	)
	require.Eventually(n.t, func() bool {
		contracts, err = n.QueryContractsFromId(wasmContractId)
		return err == nil && len(contracts) == 1
	}, time.Second*10, time.Millisecond*100)

	fmt.Printf("Finality gadget contract address: %+v", contracts)
	return contracts[0]
}
