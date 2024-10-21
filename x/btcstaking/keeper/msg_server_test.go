package keeper_test

import (
	"errors"
	"math"
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	asig "github.com/babylonlabs-io/babylon/crypto/schnorr-adaptor-signature"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/testutil/helper"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

func FuzzMsgCreateFinalityProvider(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		finalityKeeper := types.NewMockFinalityKeeper(ctrl)
		h := NewHelper(t, btclcKeeper, btccKeeper, finalityKeeper)

		// set all parameters
		h.GenAndApplyParams(r)

		// generate new finality providers
		fps := []*types.FinalityProvider{}
		for i := 0; i < int(datagen.RandomInt(r, 10)); i++ {
			fp, err := datagen.GenRandomFinalityProvider(r)
			require.NoError(t, err)
			msg := &types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: fp.Description,
				Commission:  fp.Commission,
				BtcPk:       fp.BtcPk,
				Pop:         fp.Pop,
			}
			_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
			require.NoError(t, err)

			fps = append(fps, fp)
		}
		// assert these finality providers exist in KVStore
		for _, fp := range fps {
			btcPK := *fp.BtcPk
			require.True(t, h.BTCStakingKeeper.HasFinalityProvider(h.Ctx, btcPK))
		}

		// duplicated finality providers should not pass
		for _, fp2 := range fps {
			msg := &types.MsgCreateFinalityProvider{
				Addr:        fp2.Addr,
				Description: fp2.Description,
				Commission:  fp2.Commission,
				BtcPk:       fp2.BtcPk,
				Pop:         fp2.Pop,
			}
			_, err := h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
			require.Error(t, err)
		}
	})
}

func FuzzMsgEditFinalityProvider(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		h := testhelper.NewHelper(t)
		bsKeeper := h.App.BTCStakingKeeper
		msgSrvr := keeper.NewMsgServerImpl(bsKeeper)

		// generate new finality provider
		fp, err := datagen.GenRandomFinalityProvider(r)
		require.NoError(t, err)
		// insert the finality provider
		h.AddFinalityProvider(fp)
		// assert the finality providers exist in KVStore
		require.True(t, bsKeeper.HasFinalityProvider(h.Ctx, *fp.BtcPk))

		// updated commission and description
		newCommission := datagen.GenRandomCommission(r)
		newDescription := datagen.GenRandomDescription(r)

		// scenario 1: editing finality provider should succeed
		msg := &types.MsgEditFinalityProvider{
			Addr:        fp.Addr,
			BtcPk:       *fp.BtcPk,
			Description: newDescription,
			Commission:  &newCommission,
		}
		_, err = msgSrvr.EditFinalityProvider(h.Ctx, msg)
		h.NoError(err)
		editedFp, err := bsKeeper.GetFinalityProvider(h.Ctx, *fp.BtcPk)
		h.NoError(err)
		require.Equal(t, newCommission, *editedFp.Commission)
		require.Equal(t, newDescription, editedFp.Description)

		// scenario 2: message from an unauthorised signer should fail
		newCommission = datagen.GenRandomCommission(r)
		newDescription = datagen.GenRandomDescription(r)
		invalidAddr := datagen.GenRandomAccount().Address
		msg = &types.MsgEditFinalityProvider{
			Addr:        invalidAddr,
			BtcPk:       *fp.BtcPk,
			Description: newDescription,
			Commission:  &newCommission,
		}
		_, err = msgSrvr.EditFinalityProvider(h.Ctx, msg)
		h.EqualError(err, status.Errorf(codes.PermissionDenied, "the signer does not correspond to the finality provider's Babylon address"))
		errStatus := status.Convert(err)
		require.Equal(t, codes.PermissionDenied, errStatus.Code())
	})
}

func FuzzCreateBTCDelegation(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		finalityKeeper := types.NewMockFinalityKeeper(ctrl)
		h := NewHelper(t, btclcKeeper, btccKeeper, finalityKeeper)

		// set all parameters
		h.GenAndApplyParams(r)

		changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
		require.NoError(t, err)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		usePreApproval := datagen.OneInN(r, 2)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, _, _, _, err := h.CreateDelegation(
			r,
			delSK,
			fpPK,
			changeAddress.EncodeAddress(),
			stakingValue,
			1000,
			0,
			0,
			usePreApproval,
		)
		h.NoError(err)

		// ensure consistency between the msg and the BTC delegation in DB
		actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		require.Equal(h.t, msgCreateBTCDel.StakerAddr, actualDel.StakerAddr)
		require.Equal(h.t, msgCreateBTCDel.Pop, actualDel.Pop)
		require.Equal(h.t, msgCreateBTCDel.StakingTx, actualDel.StakingTx)
		require.Equal(h.t, msgCreateBTCDel.SlashingTx, actualDel.SlashingTx)
		// ensure the BTC delegation in DB is correctly formatted
		err = actualDel.ValidateBasic()
		h.NoError(err)
		// delegation is not activated by covenant yet
		require.False(h.t, actualDel.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))

		if usePreApproval {
			require.Zero(h.t, actualDel.StartHeight)
			require.Zero(h.t, actualDel.EndHeight)
		} else {
			require.Positive(h.t, actualDel.StartHeight)
			require.Positive(h.t, actualDel.EndHeight)
		}
	})
}

func TestProperVersionInDelegation(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	finalityKeeper := types.NewMockFinalityKeeper(ctrl)
	h := NewHelper(t, btclcKeeper, btccKeeper, finalityKeeper)

	// set all parameters
	h.GenAndApplyParams(r)

	changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
	require.NoError(t, err)

	// generate and insert new finality provider
	_, fpPK, _ := h.CreateFinalityProvider(r)

	// generate and insert new BTC delegation
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	stakingTxHash, _, _, _, _, err := h.CreateDelegation(
		r,
		delSK,
		fpPK,
		changeAddress.EncodeAddress(),
		stakingValue,
		1000,
		0,
		0,
		false,
	)
	h.NoError(err)

	// ensure consistency between the msg and the BTC delegation in DB
	actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	err = actualDel.ValidateBasic()
	h.NoError(err)
	// Current version will be `1` as:
	// - version `0` is initialized by `NewHelper`
	// - version `1` is set by `GenAndApplyParams`
	require.Equal(t, uint32(1), actualDel.ParamsVersion)

	customMinUnbondingTime := uint32(2000)
	currentParams := h.BTCStakingKeeper.GetParams(h.Ctx)
	currentParams.MinUnbondingTimeBlocks = 2000
	// Update new params
	err = h.BTCStakingKeeper.SetParams(h.Ctx, currentParams)
	require.NoError(t, err)
	// create new delegation
	stakingTxHash1, _, _, _, _, err := h.CreateDelegation(
		r,
		delSK,
		fpPK,
		changeAddress.EncodeAddress(),
		stakingValue,
		1000,
		stakingValue-1000,
		uint16(customMinUnbondingTime)+1,
		false,
	)
	h.NoError(err)
	actualDel1, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash1)
	h.NoError(err)
	err = actualDel1.ValidateBasic()
	h.NoError(err)
	// Assert that the new delegation has the updated params version
	require.Equal(t, uint32(2), actualDel1.ParamsVersion)
}

func FuzzAddCovenantSigs(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		finalityKeeper := types.NewMockFinalityKeeper(ctrl)
		h := NewHelper(t, btclcKeeper, btccKeeper, finalityKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
		require.NoError(t, err)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		usePreApproval := datagen.OneInN(r, 2)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, _, _, _, err := h.CreateDelegation(
			r,
			delSK,
			fpPK,
			changeAddress.EncodeAddress(),
			stakingValue,
			1000,
			0,
			0,
			usePreApproval,
		)
		h.NoError(err)

		// ensure consistency between the msg and the BTC delegation in DB
		actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		// delegation is not activated by covenant yet
		require.False(h.t, actualDel.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))

		msgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, msgCreateBTCDel, actualDel)

		// ensure the system does not panick due to a bogus covenant sig request
		bogusMsg := *msgs[0]
		bogusMsg.StakingTxHash = datagen.GenRandomBtcdHash(r).String()
		_, err = h.MsgServer.AddCovenantSigs(h.Ctx, &bogusMsg)
		h.Error(err)

		for i, msg := range msgs {
			_, err = h.MsgServer.AddCovenantSigs(h.Ctx, msg)
			h.NoError(err)
			// check that submitting the same covenant signature returns error
			_, err = h.MsgServer.AddCovenantSigs(h.Ctx, msg)
			h.Error(err, "i: %d", i)
		}

		// ensure the BTC delegation now has voting power
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		require.True(h.t, actualDel.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))
		require.True(h.t, actualDel.BtcUndelegation.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))

		tipHeight := h.BTCLightClientKeeper.GetTipInfo(h.Ctx).Height
		checkpointTimeout := h.BTCCheckpointKeeper.GetParams(h.Ctx).CheckpointFinalizationTimeout
		covenantQuorum := h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum
		status := actualDel.GetStatus(tipHeight, checkpointTimeout, covenantQuorum)
		votingPower := actualDel.VotingPower(tipHeight, checkpointTimeout, covenantQuorum)

		if usePreApproval {
			require.Equal(t, status, types.BTCDelegationStatus_VERIFIED)
			require.Zero(t, votingPower)
		} else {
			require.Equal(t, status, types.BTCDelegationStatus_ACTIVE)
			require.Equal(t, uint64(stakingValue), votingPower)
		}
	})
}

func FuzzAddBTCDelegationInclusionProof(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		finalityKeeper := types.NewMockFinalityKeeper(ctrl)
		h := NewHelper(t, btclcKeeper, btccKeeper, finalityKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)
		changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
		require.NoError(t, err)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, err := h.CreateDelegation(
			r,
			delSK,
			fpPK,
			changeAddress.EncodeAddress(),
			stakingValue,
			1000,
			0,
			0,
			true,
		)
		h.NoError(err)

		// add covenant signatures to this BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel)

		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)

		// ensure the BTC delegation is now verified and does not have voting power
		tipHeight := h.BTCLightClientKeeper.GetTipInfo(h.Ctx).Height
		checkpointTimeout := h.BTCCheckpointKeeper.GetParams(h.Ctx).CheckpointFinalizationTimeout
		covenantQuorum := h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum
		status := actualDel.GetStatus(tipHeight, checkpointTimeout, covenantQuorum)
		votingPower := actualDel.VotingPower(tipHeight, checkpointTimeout, covenantQuorum)

		require.Equal(t, status, types.BTCDelegationStatus_VERIFIED)
		require.Zero(t, votingPower)

		// activate the BTC delegation, such that the BTC delegation becomes active
		// and has voting power
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof)

		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		status = actualDel.GetStatus(tipHeight, checkpointTimeout, covenantQuorum)
		votingPower = actualDel.VotingPower(tipHeight, checkpointTimeout, covenantQuorum)

		require.Equal(t, status, types.BTCDelegationStatus_ACTIVE)
		require.Equal(t, uint64(stakingValue), votingPower)
	})
}

func FuzzBTCUndelegate(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		finalityKeeper := types.NewMockFinalityKeeper(ctrl)
		h := NewHelper(t, btclcKeeper, btccKeeper, finalityKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)
		wValue := h.BTCCheckpointKeeper.GetParams(h.Ctx).CheckpointFinalizationTimeout

		changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
		require.NoError(t, err)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, err := h.CreateDelegation(
			r,
			delSK,
			fpPK,
			changeAddress.EncodeAddress(),
			stakingValue,
			1000,
			0,
			0,
			true,
		)
		h.NoError(err)

		// add covenant signatures to this BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel)
		// activate the BTC delegation
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof)

		// ensure the BTC delegation is bonded right now
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		btcTip := h.BTCLightClientKeeper.GetTipInfo(h.Ctx).Height
		status := actualDel.GetStatus(btcTip, wValue, bsParams.CovenantQuorum)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

		// construct unbonding msg
		delUnbondingSig, err := actualDel.SignUnbondingTx(&bsParams, h.Net, delSK)
		h.NoError(err)
		msg := &types.MsgBTCUndelegate{
			Signer:         datagen.GenRandomAccount().Address,
			StakingTxHash:  stakingTxHash,
			UnbondingTxSig: bbn.NewBIP340SignatureFromBTCSig(delUnbondingSig),
		}

		// ensure the system does not panick due to a bogus unbonding msg
		bogusMsg := *msg
		bogusMsg.StakingTxHash = datagen.GenRandomBtcdHash(r).String()
		_, err = h.MsgServer.BTCUndelegate(h.Ctx, &bogusMsg)
		h.Error(err)

		// unbond
		_, err = h.MsgServer.BTCUndelegate(h.Ctx, msg)
		h.NoError(err)

		// ensure the BTC delegation is unbonded
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		status = actualDel.GetStatus(btcTip, wValue, bsParams.CovenantQuorum)
		require.Equal(t, types.BTCDelegationStatus_UNBONDED, status)
	})
}

func FuzzSelectiveSlashing(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		finalityKeeper := types.NewMockFinalityKeeper(ctrl)
		h := NewHelper(t, btclcKeeper, btccKeeper, finalityKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)
		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
		require.NoError(t, err)

		// generate and insert new finality provider
		fpSK, fpPK, _ := h.CreateFinalityProvider(r)
		fpBtcPk := bbn.NewBIP340PubKeyFromBTCPK(fpPK)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, err := h.CreateDelegation(
			r,
			delSK,
			fpPK,
			changeAddress.EncodeAddress(),
			stakingValue,
			1000,
			0,
			0,
			true,
		)
		h.NoError(err)

		// add covenant signatures to this BTC delegation
		// so that the BTC delegation becomes bonded
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel)
		// activate the BTC delegation
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof)

		// now BTC delegation has all covenant signatures
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		require.True(t, actualDel.HasCovenantQuorums(bsParams.CovenantQuorum))

		// construct message for the evidence of selective slashing
		msg := &types.MsgSelectiveSlashingEvidence{
			Signer:           datagen.GenRandomAccount().Address,
			StakingTxHash:    actualDel.MustGetStakingTxHash().String(),
			RecoveredFpBtcSk: fpSK.Serialize(),
		}

		// ensure the system does not panick due to a bogus unbonding msg
		bogusMsg := *msg
		bogusMsg.StakingTxHash = datagen.GenRandomBtcdHash(r).String()
		_, err = h.MsgServer.SelectiveSlashingEvidence(h.Ctx, &bogusMsg)
		h.Error(err)

		// submit evidence of selective slashing
		_, err = h.MsgServer.SelectiveSlashingEvidence(h.Ctx, msg)
		h.NoError(err)

		// ensure the finality provider is slashed
		slashedFp, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fpBtcPk.MustMarshal())
		h.NoError(err)
		require.True(t, slashedFp.IsSlashed())
	})
}

func FuzzSelectiveSlashing_StakingTx(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		finalityKeeper := types.NewMockFinalityKeeper(ctrl)
		h := NewHelper(t, btclcKeeper, btccKeeper, finalityKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)
		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
		require.NoError(t, err)

		// generate and insert new finality provider
		fpSK, fpPK, _ := h.CreateFinalityProvider(r)
		fpBtcPk := bbn.NewBIP340PubKeyFromBTCPK(fpPK)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, err := h.CreateDelegation(
			r,
			delSK,
			fpPK,
			changeAddress.EncodeAddress(),
			stakingValue,
			1000,
			0,
			0,
			true,
		)
		h.NoError(err)

		// add covenant signatures to this BTC delegation
		// so that the BTC delegation becomes bonded
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel)
		// activate the BTC delegation
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof)
		// now BTC delegation has all covenant signatures
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		require.True(t, actualDel.HasCovenantQuorums(bsParams.CovenantQuorum))

		// finality provider pulls off selective slashing by decrypting covenant's adaptor signature
		// on the slashing tx
		// choose a random covenant adaptor signature
		covIdx := datagen.RandomInt(r, int(bsParams.CovenantQuorum))
		covPK := bbn.NewBIP340PubKeyFromBTCPK(covenantSKs[covIdx].PubKey())
		fpIdx := datagen.RandomInt(r, len(actualDel.FpBtcPkList))
		covASig, err := actualDel.GetCovSlashingAdaptorSig(covPK, int(fpIdx), bsParams.CovenantQuorum)
		h.NoError(err)

		// finality provider decrypts the covenant signature
		decKey, err := asig.NewDecyptionKeyFromBTCSK(fpSK)
		h.NoError(err)
		decryptedCovenantSig := bbn.NewBIP340SignatureFromBTCSig(covASig.Decrypt(decKey))

		// recover the fpSK by using adaptor signature and decrypted Schnorr signature
		recoveredFPDecKey := covASig.Recover(decryptedCovenantSig.MustToBTCSig())
		recoveredFPSK := recoveredFPDecKey.ToBTCSK()
		// ensure the recovered finality provider SK is same as the real one
		require.Equal(t, fpSK.Serialize(), recoveredFPSK.Serialize())

		// submit evidence of selective slashing
		msg := &types.MsgSelectiveSlashingEvidence{
			Signer:           datagen.GenRandomAccount().Address,
			StakingTxHash:    actualDel.MustGetStakingTxHash().String(),
			RecoveredFpBtcSk: recoveredFPSK.Serialize(),
		}
		_, err = h.MsgServer.SelectiveSlashingEvidence(h.Ctx, msg)
		h.NoError(err)

		// ensure the finality provider is slashed
		slashedFp, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fpBtcPk.MustMarshal())
		h.NoError(err)
		require.True(t, slashedFp.IsSlashed())
	})
}

func TestDoNotAllowDelegationWithoutFinalityProvider(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	btccKeeper.EXPECT().GetParams(gomock.Any()).Return(btcctypes.DefaultParams()).AnyTimes()
	finalityKeeper := types.NewMockFinalityKeeper(ctrl)
	h := NewHelper(t, btclcKeeper, btccKeeper, finalityKeeper)

	// set covenant PK to params
	_, covenantPKs := h.GenAndApplyParams(r)
	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)
	bcParams := h.BTCCheckpointKeeper.GetParams(h.Ctx)

	minUnbondingTime := types.MinimumUnbondingTime(
		&bsParams,
		&bcParams,
	)

	slashingChangeLockTime := uint16(minUnbondingTime) + 1

	// We only generate a finality provider, but not insert it into KVStore. So later
	// insertion of delegation should fail.
	_, fpPK, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	/*
		generate and insert valid new BTC delegation
	*/
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	stakingTimeBlocks := uint16(5)
	stakingValue := int64(2 * 10e8)
	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r,
		t,
		h.Net,
		delSK,
		[]*btcec.PublicKey{fpPK},
		covenantPKs,
		bsParams.CovenantQuorum,
		stakingTimeBlocks,
		stakingValue,
		bsParams.SlashingPkScript,
		bsParams.SlashingRate,
		slashingChangeLockTime,
	)
	// get msgTx
	stakingMsgTx := testStakingInfo.StakingTx
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingMsgTx)
	require.NoError(t, err)
	// random Babylon SK
	acc := datagen.GenRandomAccount()
	stakerAddr := sdk.MustAccAddressFromBech32(acc.Address)

	// PoP
	pop, err := types.NewPoPBTC(stakerAddr, delSK)
	require.NoError(t, err)
	// generate staking tx info
	prevBlock, _ := datagen.GenRandomBtcdBlock(r, 0, nil)
	btcHeaderWithProof := datagen.CreateBlockWithTransaction(r, &prevBlock.Header, stakingMsgTx)
	btcHeader := btcHeaderWithProof.HeaderBytes
	txInclusionProof := types.NewInclusionProof(
		&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()},
		btcHeaderWithProof.SpvProof.MerkleNodes,
	)

	slashingPathInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// generate proper delegator sig
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		stakingMsgTx,
		0,
		slashingPathInfo.GetPkScriptPath(),
		delSK,
	)
	require.NoError(t, err)

	stkTxHash := testStakingInfo.StakingTx.TxHash()
	unbondingTime := 100 + 1
	unbondingValue := stakingValue - datagen.UnbondingTxFee // TODO: parameterise fee
	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		h.Net,
		delSK,
		[]*btcec.PublicKey{fpPK},
		covenantPKs,
		bsParams.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		uint16(unbondingTime),
		unbondingValue,
		bsParams.SlashingPkScript,
		bsParams.SlashingRate,
		slashingChangeLockTime,
	)
	unbondingTx, err := bbn.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	h.NoError(err)
	delUnbondingSlashingSig, err := testUnbondingInfo.GenDelSlashingTxSig(delSK)
	h.NoError(err)

	// all good, construct and send MsgCreateBTCDelegation message
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(fpPK)
	msgCreateBTCDel := &types.MsgCreateBTCDelegation{
		StakerAddr:                    stakerAddr.String(),
		FpBtcPkList:                   []bbn.BIP340PubKey{*fpBTCPK},
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(delSK.PubKey()),
		Pop:                           pop,
		StakingTime:                   uint32(stakingTimeBlocks),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		StakingTxInclusionProof:       txInclusionProof,
		SlashingTx:                    testStakingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingTx:                   unbondingTx,
		UnbondingTime:                 uint32(unbondingTime),
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           testUnbondingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSlashingSig,
	}

	_, err = h.MsgServer.CreateBTCDelegation(h.Ctx, msgCreateBTCDel)
	require.Error(t, err)
	require.True(t, errors.Is(err, types.ErrFpNotFound))
}

func TestCorrectUnbondingTimeInDelegation(t *testing.T) {
	tests := []struct {
		name                      string
		finalizationTimeout       uint32
		minUnbondingTime          uint32
		unbondingTimeInDelegation uint16
		err                       error
	}{
		{
			name:                      "successful delegation when ubonding time in delegation is larger than finalization timeout when finalization timeout is larger than min unbonding time",
			unbondingTimeInDelegation: 101,
			minUnbondingTime:          99,
			finalizationTimeout:       100,
			err:                       nil,
		},
		{
			name:                      "failed delegation when ubonding time in delegation is not larger than finalization time when min unbonding time is lower than finalization timeout",
			unbondingTimeInDelegation: 100,
			minUnbondingTime:          99,
			finalizationTimeout:       100,
			err:                       types.ErrInvalidUnbondingTx,
		},
		{
			name:                      "successful delegation when ubonding time ubonding time in delegation is larger than min unbonding time when min unbonding time is larger than finalization timeout",
			unbondingTimeInDelegation: 151,
			minUnbondingTime:          150,
			finalizationTimeout:       100,
			err:                       nil,
		},
		{
			name:                      "failed delegation when ubonding time in delegation is not larger than minUnbondingTime when min unbonding time is larger than finalization timeout",
			unbondingTimeInDelegation: 150,
			minUnbondingTime:          150,
			finalizationTimeout:       100,
			err:                       types.ErrInvalidUnbondingTx,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := rand.New(rand.NewSource(time.Now().Unix()))
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// mock BTC light client and BTC checkpoint modules
			btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
			btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
			finalityKeeper := types.NewMockFinalityKeeper(ctrl)
			h := NewHelper(t, btclcKeeper, btccKeeper, finalityKeeper)

			// set all parameters
			_, _ = h.GenAndApplyCustomParams(r, tt.finalizationTimeout, tt.minUnbondingTime)

			changeAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
			require.NoError(t, err)

			// generate and insert new finality provider
			_, fpPK, _ := h.CreateFinalityProvider(r)

			// generate and insert new BTC delegation
			stakingValue := int64(2 * 10e8)
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)
			stakingTxHash, _, _, _, _, err := h.CreateDelegation(
				r,
				delSK,
				fpPK,
				changeAddress.EncodeAddress(),
				stakingValue,
				1000,
				stakingValue-1000,
				tt.unbondingTimeInDelegation,
				true,
			)
			if tt.err != nil {
				require.Error(t, err)
				require.True(t, errors.Is(err, tt.err))
			} else {
				require.NoError(t, err)
				// Retrieve delegation from keeper
				delegation, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
				require.NoError(t, err)
				require.Equal(t, tt.unbondingTimeInDelegation, uint16(delegation.UnbondingTime))
			}
		})
	}
}

func createNDelegationsForFinalityProvider(
	r *rand.Rand,
	t *testing.T,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	numDelegations int,
	quorum uint32,
) []*types.BTCDelegation {
	var delegations []*types.BTCDelegation
	for i := 0; i < numDelegations; i++ {
		covenatnSks, covenantPks, err := datagen.GenRandomBTCKeyPairs(r, int(quorum))
		require.NoError(t, err)

		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)

		slashingAddress, err := datagen.GenRandomBTCAddress(r, net)
		require.NoError(t, err)
		slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
		require.NoError(t, err)

		slashingRate := sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2)

		startHeight, endHeight := 1, math.MaxUint16
		stakingTime := uint32(endHeight) - uint32(startHeight)
		del, err := datagen.GenRandomBTCDelegation(
			r,
			t,
			net,
			[]bbn.BIP340PubKey{*bbn.NewBIP340PubKeyFromBTCPK(fpPK)},
			delSK,
			covenatnSks,
			covenantPks,
			quorum,
			slashingPkScript,
			stakingTime,
			1,
			1+(math.MaxUint16-1),
			uint64(stakingValue),
			slashingRate,
			math.MaxUint16,
		)
		require.NoError(t, err)

		delegations = append(delegations, del)
	}
	return delegations
}
