package e2e2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	v47 "github.com/babylonlabs-io/babylon/v4/app/upgrades/v4_7"
	"github.com/babylonlabs-io/babylon/v4/test/e2ev2/tmanager"
)

// TestUpgradeV47 coordinates the Go 1.26.10 binary swap at the v4.7 plan height.
// Gov and grant-free authz exercise both gated and ungated URL parser changes.
func TestUpgradeV47(t *testing.T) {
	t.Parallel()

	tm := tmanager.NewTmWithUpgrade(t, 0, "v4.6.0-testnet")
	chain := tm.Chains[tmanager.CHAIN_ID_BABYLON]
	n := chain.Nodes[0]
	tm.Start()
	n.WaitUntilBlkHeight(3)
	btcHeight := n.QueryBtcStakingParams().BtcActivationHeight
	paramsBefore, versionBefore := n.QueryBtcStakingParamsByBTCHeight(btcHeight)

	code := []byte("\x00asm\x01\x00\x00\x00")
	codeHash := sha256.Sum256(code)
	for _, upgraded := range []bool{false, true} {
		for _, source := range []string{"https://example.com/x", "https://a:1:2/x", "https://[foo]/x"} {
			msg, err := types.NewAnyWithValue(&wasmtypes.MsgStoreAndInstantiateContract{
				Authority: appparams.AccGov.String(), WASMByteCode: code,
				Label: "go126-probe", Msg: []byte("{}"), Source: source,
				Builder: "cosmwasm/optimizer:0.16.0", CodeHash: codeHash[:],
			})
			require.NoError(t, err)
			for _, grantFree := range []bool{false, true} {
				wallet := n.DefaultWallet()
				var txMsg sdk.Msg = &govtypes.MsgSubmitProposal{
					Messages: []*types.Any{msg}, Proposer: wallet.Addr(),
					InitialDeposit: sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(1_000_000))),
					Title:          "Go upgrade probe", Summary: source,
				}
				wantCode := uint32(0)
				if grantFree {
					txMsg = &authz.MsgExec{Grantee: wallet.Addr(), Msgs: []*types.Any{msg}}
					wantCode = 2 // authz.ErrNoAuthorizationFound
				}
				if upgraded && source != "https://example.com/x" {
					wantCode = 12 // gov.ErrInvalidProposalMsg
					if grantFree {
						wantCode = 1 // undefined: wasm URL validation error
					}
				}
				hash, err := n.SubmitTx(wallet.SignMsgWithGas(1_000_000, txMsg))
				require.NoError(t, err)
				txHash, err := hex.DecodeString(hash)
				require.NoError(t, err)
				var first *coretypes.ResultTx
				var appHash []byte
				for _, node := range chain.AllNodes() {
					var result *coretypes.ResultTx
					var block *coretypes.ResultBlock
					require.Eventually(t, func() bool {
						result, err = node.RpcClient.Tx(context.Background(), txHash, false)
						if err != nil {
							return false
						}
						nextHeight := result.Height + 1
						block, err = node.RpcClient.Block(context.Background(), &nextHeight)
						return err == nil
					}, time.Minute, time.Second, "node %s did not commit tx %s and its successor block", node.Name, hash)
					require.Equal(t, wantCode, result.TxResult.Code, "upgraded=%v authz=%v source=%s: %s", upgraded, grantFree, source, result.TxResult.Log)
					if first == nil {
						first, appHash = result, block.Block.AppHash
						continue
					}
					require.Equal(t, first.Height, result.Height)
					require.Equal(t, first.TxResult.Data, result.TxResult.Data)
					require.Equal(t, first.TxResult.GasWanted, result.TxResult.GasWanted)
					require.Equal(t, first.TxResult.GasUsed, result.TxResult.GasUsed)
					require.Equal(t, appHash, []byte(block.Block.AppHash))
				}
			}
		}
		if !upgraded {
			govMsg, preUpgrade := createGovPropAndPreUpgradeFunc(t, tm.ChainValidator().Wallet.WalletSender, v47.UpgradeName, 0)
			tm.Upgrade(govMsg, preUpgrade)
		}
	}

	// btcstaking v2 moves the height-to-parameter-version key during RunMigrations.
	paramsAfter, versionAfter := n.QueryBtcStakingParamsByBTCHeight(btcHeight)
	require.Equal(t, versionBefore, versionAfter)
	require.Equal(t, paramsBefore, paramsAfter)

	// Ordinary transactions still work after the coordinated swap.
	recipient := n.CreateWallet("recipient")
	amount := sdk.NewCoins(sdk.NewInt64Coin(appparams.DefaultBondDenom, 1_000_000))
	n.SendCoins(recipient.Addr(), amount)
	n.WaitForNextBlock()
	require.Equal(t, amount, n.QueryAllBalances(recipient.Addr()))
}
