package replay

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bstypes "github.com/babylonlabs-io/babylon/x/btcstaking/types"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"
)

func FuzzJailing(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 5)

	f.Fuzz(func(t *testing.T, seed int64) {
		t.Parallel()
		r := rand.New(rand.NewSource(seed))
		numFinalityProviders := uint32(datagen.RandomInRange(r, 5, 7))
		numDelPerFp := uint32(2)
		driverTempDir := t.TempDir()
		replayerTempDir := t.TempDir()
		driver := NewBabylonAppDriver(t, driverTempDir, replayerTempDir)

		driver.GenerateNewBlock(t)

		stakingParams := driver.GetBTCStakingParams(t)

		fpInfos := GenerateNFinalityProviders(
			r,
			t,
			numFinalityProviders,
			driver.GetDriverAccountAddress(),
		)
		registerMsgs := FpInfosToMsgs(fpInfos)
		driver.SendTxWithMsgsFromDriverAccount(t, registerMsgs...)

		var msgList []*ftypes.MsgCommitPubRandList
		for _, fpInfo := range fpInfos {
			_, msg, err := datagen.GenRandomMsgCommitPubRandList(r, fpInfo.BTCPrivateKey, 1, 1000)
			require.NoError(t, err)
			msg.Signer = driver.GetDriverAccountAddress().String()
			msgList = append(msgList, msg)
		}
		// send all commit randomness messages in one block
		driver.SendTxWithMsgsFromDriverAccount(t, MsgsToSdkMsg(msgList)...)
		currnetEpochNunber := driver.GetEpoch().EpochNumber
		driver.ProgressTillFirstBlockTheNextEpoch(t)

		driver.FinializeCkptForEpoch(r, t, currnetEpochNunber)

		// at this point randomness is finalized.
		// - send delegations
		// - send covenant signatures
		// - send delegation inclusion proofs
		var allDelegationInfos []*datagen.CreateDelegationInfo

		for _, fpInfo := range fpInfos {
			delInfos := GenerateNBTCDelegationsForFinalityProvider(
				r,
				t,
				numDelPerFp,
				driver.GetDriverAccountAddress(),
				fpInfo,
				stakingParams,
			)
			allDelegationInfos = append(allDelegationInfos, delInfos...)
		}

		createDelegationMsgs := ToCreateBTCDelegationMsgs(allDelegationInfos)
		covenantSignaturesMsgs := ToCovenantSignaturesMsgs(allDelegationInfos)
		driver.SendTxWithMsgsFromDriverAccount(t, createDelegationMsgs...)
		driver.SendTxWithMsgsFromDriverAccount(t, covenantSignaturesMsgs...)

		// all delegations are verified after activation finality provider should
		// have voting power
		stakingTransactions := DelegationInfosToBTCTx(allDelegationInfos)
		blockWithProofs := driver.GenBlockWithTransactions(
			r,
			t,
			stakingTransactions,
		)
		// make staking txs k-deep
		driver.ExtendBTCLcWithNEmptyBlocks(r, t, 10)

		activationMsgs := BlockWithProofsToActivationMessages(blockWithProofs, driver.GetDriverAccountAddress())

		activeFps := driver.GetActiveFpsAtCurrentHeight(t)
		require.Equal(t, 0, len(activeFps))

		driver.SendTxWithMsgsFromDriverAccount(t, activationMsgs...)

		// on the last block all power events were queued for execution
		// after this block execution they should be processed and our fps should
		// have voting power
		driver.GenerateNewBlock(t)
		activeFps = driver.GetActiveFpsAtCurrentHeight(t)
		require.Equal(t, numFinalityProviders, uint32(len(activeFps)))

		// we do not have voting in this test, so wait until all fps are jailed
		driver.WaitTillAllFpsJailed(t)
		driver.GenerateNewBlock(t)
		activeFps = driver.GetActiveFpsAtCurrentHeight(t)
		require.Equal(t, 0, len(activeFps))

		// Replay all the blocks from driver and check appHash
		replayer := NewBlockReplayer(t, replayerTempDir)
		replayer.ReplayBlocks(t, driver.FinalizedBlocks)
		// after replay we should have the same apphash
		require.Equal(t, driver.LastState.LastBlockHeight, replayer.LastState.LastBlockHeight)
		require.Equal(t, driver.LastState.AppHash, replayer.LastState.AppHash)
	})
}

func BlockWithProofsToActivationMessages(
	blockWithProofs *datagen.BlockWithProofs,
	senderAddr sdk.AccAddress,
) []sdk.Msg {
	msgs := []sdk.Msg{}

	for i, tx := range blockWithProofs.Transactions {
		// no coinbase tx
		if i == 0 {
			continue
		}

		msgs = append(msgs, &bstypes.MsgAddBTCDelegationInclusionProof{
			Signer:                  senderAddr.String(),
			StakingTxHash:           tx.TxHash().String(),
			StakingTxInclusionProof: bstypes.NewInclusionProofFromSpvProof(blockWithProofs.Proofs[i]),
		})
	}
	return msgs
}
