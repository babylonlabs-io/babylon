package keeper_test

import (
	"encoding/hex"
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
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	asig "github.com/babylonlabs-io/babylon/v3/crypto/schnorr-adaptor-signature"
	testutil "github.com/babylonlabs-io/babylon/v3/testutil/btcstaking-helper"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	testhelper "github.com/babylonlabs-io/babylon/v3/testutil/helper"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	btcctypes "github.com/babylonlabs-io/babylon/v3/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v3/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	btcsctypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
)

func FuzzMsgServer_UpdateParams(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 500)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		h.GenAndApplyParams(r)

		params := h.BTCStakingKeeper.GetParams(h.Ctx)
		ckptFinalizationTimeout := btccKeeper.GetParams(h.Ctx).CheckpointFinalizationTimeout
		params.UnbondingTimeBlocks = uint32(r.Intn(int(ckptFinalizationTimeout))) + 1
		params.BtcActivationHeight++

		// Try to update params with minUnbondingTime less than or equal to checkpointFinalizationTimeout
		msg := &types.MsgUpdateParams{
			Authority: appparams.AccGov.String(),
			Params:    params,
		}

		_, err := h.MsgServer.UpdateParams(h.Ctx, msg)
		require.ErrorIs(t, err, govtypes.ErrInvalidProposalMsg,
			"should not set minUnbondingTime to be less than checkpointFinalizationTimeout")

		// Try to update params with minUnbondingTime larger than checkpointFinalizationTimeout
		msg.Params.UnbondingTimeBlocks = uint32(r.Intn(1000)) + ckptFinalizationTimeout + 1
		_, err = h.MsgServer.UpdateParams(h.Ctx, msg)
		require.NoError(t, err)
	})
}

func FuzzMsgCreateFinalityProvider(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		h.GenAndApplyParams(r)

		// Define BSN IDs
		unregisteredBsnId := "unregistered-bsn-" + datagen.GenRandomHexStr(r, 10)
		registeredBsnId := "registered-bsn-" + datagen.GenRandomHexStr(r, 10)
		babylonBsnId := h.Ctx.ChainID()

		// Register one additional BSN
		// TODO: Use a mock BSC keeper instead of creating real consumers
		// Create a random consumer name
		consumerName := datagen.GenRandomHexStr(r, 5)
		// Create a random consumer description
		consumerDesc := "Consumer description: " + datagen.GenRandomHexStr(r, 15)

		// Populate ConsumerRegister object
		consumerRegister := &btcsctypes.ConsumerRegister{
			ConsumerId:          registeredBsnId,
			ConsumerName:        consumerName,
			ConsumerDescription: consumerDesc,
		}

		// Register the consumer
		err := h.BTCStkConsumerKeeper.RegisterConsumer(h.Ctx, consumerRegister)
		require.NoError(t, err)

		// Register a finality provider to an unregistered BSN should fail
		fpUnregisteredBsn, err := datagen.GenRandomFinalityProvider(r, h.FpPopContext(), unregisteredBsnId)
		require.NoError(t, err)
		msgUnregisteredBsn := &types.MsgCreateFinalityProvider{
			Addr:        fpUnregisteredBsn.Addr,
			Description: fpUnregisteredBsn.Description,
			Commission: types.NewCommissionRates(
				*fpUnregisteredBsn.Commission,
				fpUnregisteredBsn.CommissionInfo.MaxRate,
				fpUnregisteredBsn.CommissionInfo.MaxChangeRate,
			),
			BtcPk: fpUnregisteredBsn.BtcPk,
			Pop:   fpUnregisteredBsn.Pop,
			BsnId: unregisteredBsnId,
		}
		_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, msgUnregisteredBsn)
		require.Error(t, err)

		fps := []*types.FinalityProvider{}
		for i := 0; i < int(datagen.RandomInt(r, 20)); i++ {
			bsnId := ""
			if datagen.RandomInt(r, 2) == 0 {
				bsnId = registeredBsnId
			}
			fp, err := datagen.GenRandomFinalityProvider(r, h.FpPopContext(), bsnId)
			require.NoError(t, err)
			msg := &types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: fp.Description,
				Commission: types.NewCommissionRates(
					*fp.Commission,
					fp.CommissionInfo.MaxRate,
					fp.CommissionInfo.MaxChangeRate,
				),
				BtcPk: fp.BtcPk,
				Pop:   fp.Pop,
				BsnId: fp.BsnId,
			}
			_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, msg)
			require.NoError(t, err)

			fps = append(fps, fp)
		}

		// assert these finality providers exist in KVStore
		for _, fp := range fps {
			btcPK := fp.BtcPk.MustMarshal()
			require.True(t, h.BTCStakingKeeper.HasFinalityProvider(h.Ctx, btcPK))
			// Ensure that the if a finality provider creation message does not
			// contain a bsnId, then we default to the Babylon Genesis chain id.
			bsnId := fp.BsnId
			if bsnId == "" {
				bsnId = babylonBsnId
			}
			actualFp, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, fp.BtcPk.MustMarshal())
			require.NoError(t, err)
			require.Equal(t, bsnId, actualFp.BsnId)
		}

		// duplicated finality providers should not pass
		// this also implicitly tests the case in which
		// the finality provider is registered to a different BSN
		for _, fp2 := range fps {
			msg := &types.MsgCreateFinalityProvider{
				Addr:        fp2.Addr,
				Description: fp2.Description,
				Commission: types.NewCommissionRates(
					*fp2.Commission,
					fp2.CommissionInfo.MaxRate,
					fp2.CommissionInfo.MaxChangeRate,
				),
				BtcPk: fp2.BtcPk,
				Pop:   fp2.Pop,
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
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		h.GenAndApplyParams(r)

		// insert the finality provider
		_, _, fp := h.CreateFinalityProvider(r)
		// assert the finality providers exist in KVStore
		require.True(t, h.BTCStakingKeeper.HasFinalityProvider(h.Ctx, *fp.BtcPk))

		// updated commission and description
		newCommission := datagen.GenRandomCommission(r)
		newDescription := datagen.GenRandomDescription(r)

		// scenario 1: editing finality provider should succeed
		// Note that, on finality provider creation, the commission update time is set to the current block time.
		// So we need to update block time to be after 24hs to edit the commission
		h.Ctx = h.Ctx.WithBlockTime(h.Ctx.BlockTime().Add(25 * time.Hour))
		msg := &types.MsgEditFinalityProvider{
			Addr:        fp.Addr,
			BtcPk:       *fp.BtcPk,
			Description: newDescription,
			Commission:  &newCommission,
		}
		_, err := h.MsgServer.EditFinalityProvider(h.Ctx, msg)
		h.NoError(err)
		editedFp, err := h.BTCStakingKeeper.GetFinalityProvider(h.Ctx, *fp.BtcPk)
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
		_, err = h.MsgServer.EditFinalityProvider(h.Ctx, msg)
		require.Equal(h.T(), err, status.Errorf(codes.PermissionDenied, "the signer does not correspond to the finality provider's Babylon address"))
		errStatus := status.Convert(err)
		require.Equal(h.T(), codes.PermissionDenied, errStatus.Code())
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
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		h.GenAndApplyParams(r)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		usePreApproval := datagen.OneInN(r, 2)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)

		var stakingTxHash string
		var msgCreateBTCDel *types.MsgCreateBTCDelegation
		if usePreApproval {
			stakingTxHash, msgCreateBTCDel, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				[]*btcec.PublicKey{fpPK},
				stakingValue,
				1000,
				0,
				0,
				usePreApproval,
				false,
				10,
				10,
			)
			h.NoError(err)
		} else {
			stakingTxHash, msgCreateBTCDel, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				[]*btcec.PublicKey{fpPK},
				stakingValue,
				1000,
				0,
				0,
				usePreApproval,
				false,
				10,
				30,
			)
			h.NoError(err)
		}

		// ensure consistency between the msg and the BTC delegation in DB
		actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		require.Equal(h.T(), msgCreateBTCDel.StakerAddr, actualDel.StakerAddr)
		require.Equal(h.T(), msgCreateBTCDel.Pop, actualDel.Pop)
		require.Equal(h.T(), msgCreateBTCDel.StakingTx, actualDel.StakingTx)
		require.Equal(h.T(), msgCreateBTCDel.SlashingTx, actualDel.SlashingTx)
		// ensure the BTC delegation in DB is correctly formatted
		err = actualDel.ValidateBasic()
		h.NoError(err)
		// delegation is not activated by covenant yet
		require.False(h.T(), actualDel.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))

		if usePreApproval {
			require.Zero(h.T(), actualDel.StartHeight)
			require.Zero(h.T(), actualDel.EndHeight)
		} else {
			require.Positive(h.T(), actualDel.StartHeight)
			require.Positive(h.T(), actualDel.EndHeight)
		}
	})
}

func FuzzCreateBTCDelegationWithParamsFromBtcHeight(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(time.Now().Unix()))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		btcTipHeight := uint32(30)
		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		h.GenAndApplyParams(r)
		ctx, k := h.Ctx, h.BTCStakingKeeper

		versionedParams := k.GetParamsWithVersion(ctx)
		currentParams := versionedParams.Params

		maxGapBlocksBetweenParams := datagen.RandomUInt32(r, 100) + 100
		// we are adding btcTipHeight so that delegation will always be included
		// after the initial tip height
		expectedParamsBlockHeight := btcTipHeight + datagen.RandomUInt32(r, maxGapBlocksBetweenParams) + currentParams.BtcActivationHeight + 1
		expectedParamsVersion := versionedParams.Version + 1

		currentParams.BtcActivationHeight = expectedParamsBlockHeight
		err := k.SetParams(ctx, currentParams)
		require.NoError(t, err)

		nextBtcActivationHeight := btcTipHeight + datagen.RandomUInt32(r, maxGapBlocksBetweenParams) + currentParams.BtcActivationHeight + 1
		currentParams.BtcActivationHeight = nextBtcActivationHeight
		err = k.SetParams(ctx, currentParams)
		require.NoError(t, err)

		// makes sure that at the BTC block height 300 will use the expected param
		p, version, err := k.GetParamsForBtcHeight(ctx, uint64(nextBtcActivationHeight-1))
		h.NoError(err)
		require.Equal(t, p.BtcActivationHeight, expectedParamsBlockHeight)
		require.Equal(t, version, expectedParamsVersion)

		// creates one BTC delegation with BTC block height between expectedParamsBlockHeight and 500
		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		btcBlockHeight := datagen.RandomUInt32(r, nextBtcActivationHeight-expectedParamsBlockHeight) + expectedParamsBlockHeight
		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		_, _, btcDel, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK},
			stakingValue,
			1000,
			0,
			0,
			false,
			false,
			btcBlockHeight,
			btcTipHeight,
		)
		h.NoError(err)
		require.NotNil(t, btcDel.ParamsVersion, expectedParamsVersion)
	})
}

func TestProperVersionInDelegation(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

	// set all parameters
	h.GenAndApplyParams(r)

	// generate and insert new finality provider
	_, fpPK, _ := h.CreateFinalityProvider(r)

	// generate and insert new BTC delegation
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	stakingTxHash, _, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		[]*btcec.PublicKey{fpPK},
		stakingValue,
		1000,
		0,
		0,
		false,
		false,
		10,
		30,
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
	currentParams.UnbondingTimeBlocks = customMinUnbondingTime
	currentParams.BtcActivationHeight++
	// Update new params
	err = h.BTCStakingKeeper.SetParams(h.Ctx, currentParams)
	require.NoError(t, err)
	// create new delegation
	stakingTxHash1, _, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		[]*btcec.PublicKey{fpPK},
		stakingValue,
		10000,
		stakingValue-1000,
		uint16(customMinUnbondingTime),
		false,
		false,
		10,
		30,
	)
	h.NoError(err)
	actualDel1, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash1)
	h.NoError(err)
	err = actualDel1.ValidateBasic()
	h.NoError(err)
	// Assert that the new delegation has the updated params version
	require.Equal(t, uint32(2), actualDel1.ParamsVersion)
}

// TestBtcStakingWithBtcReOrg creates an BTC staking delegation
// with enough covenant signatures submitted to be considered ACTIVE.
func TestBtcStakingWithBtcReOrg(t *testing.T) {
	btcLightclientTipHeight := uint32(30)
	// btc staking tx will be included at btcLightclientTipHeight - BTC confirmation depth
	h, r, btcctParams, stakingTxHash := createActiveBtcDel(t, btcLightclientTipHeight)

	// verifies the largest reorg without anything set
	_, err := h.BTCStakingKeeper.LargestBtcReOrg(h.Ctx, &types.QueryLargestBtcReOrgRequest{})
	require.EqualError(t, err, types.ErrLargestBtcReorgNotFound.Error())

	// should not panic in end blocker since there is no reorg
	require.NotPanics(t, func() {
		_, err = btcstaking.EndBlocker(h.Ctx, *h.BTCStakingKeeper)
		h.NoError(err)
	})

	// -------------- simulates a reorg of current tip - (BTC depth - 1) --------
	// It should not panic in x/btcstaking end blocker as the reorg is at the limit allowed
	// It should consider the BTC staking as PENDING, since the block depth was revoked
	rBlockFrom, rBlockTo := datagen.GenRandomBTCHeaderInfo(r), datagen.GenRandomBTCHeaderInfo(r)
	rBlockFrom.Height = btcLightclientTipHeight
	rBlockTo.Height = btcLightclientTipHeight - (btcctParams.BtcConfirmationDepth - 1)
	currLargestReorg := types.NewLargestBtcReOrg(rBlockFrom, rBlockTo)

	err = h.BTCStakingKeeper.SetLargestBtcReorg(h.Ctx, currLargestReorg)
	h.NoError(err)

	// should not panic in end blocker since the reorg is less than the allowed
	require.NotPanics(t, func() {
		_, err = btcstaking.EndBlocker(h.Ctx, *h.BTCStakingKeeper)
		h.NoError(err)
	})

	// checks the query with a reorg set
	respLargestReOrg, err := h.BTCStakingKeeper.LargestBtcReOrg(h.Ctx, &types.QueryLargestBtcReOrgRequest{})
	h.NoError(err)
	require.Equal(t, respLargestReOrg.BlockDiff, currLargestReorg.BlockDiff)
	require.Equal(t, respLargestReOrg.RollbackFrom.HashHex, rBlockFrom.ToResponse().HashHex)
	require.Equal(t, respLargestReOrg.RollbackTo.HashHex, rBlockTo.ToResponse().HashHex)

	// BTC staking tx is still seen as active rolling back to a block where the confirmation depth is less than btcctParams.BtcConfirmationDepth
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: rBlockTo.Height})
	delResp, err := h.BTCStakingKeeper.BTCDelegation(h.Ctx, &types.QueryBTCDelegationRequest{
		StakingTxHashHex: stakingTxHash,
	})
	h.NoError(err)
	require.Equal(t, types.BTCDelegationStatus_ACTIVE.String(), delResp.BtcDelegation.StatusDesc)

	// -------------- simulates a reorg of current tip - (BTC depth) --------
	// Should panic in x/btcstaking end blocker as the reorg is the size of k'
	// If a big reorg happened each btc staking transaction included in this last reorg blocks
	// will need to be analyzed if they are included in the new reorganization of blocks
	// and a emergency upgrade will be needed to revoke this values stored in voting power and rewards
	rBlockFrom.Height = btcLightclientTipHeight
	rBlockTo.Height = btcLightclientTipHeight - (btcctParams.BtcConfirmationDepth)
	currLargestReorg = types.NewLargestBtcReOrg(rBlockFrom, rBlockTo)

	err = h.BTCStakingKeeper.SetLargestBtcReorg(h.Ctx, currLargestReorg)
	h.NoError(err)

	// should panic in end blocker since the reorg is the size of BTC Confirmation Depth
	require.Panics(t, func() {
		_, err = btcstaking.EndBlocker(h.Ctx, *h.BTCStakingKeeper)
		h.NoError(err)
	})

	// verifies the query of the largest reorg again
	respLargestReOrg, err = h.BTCStakingKeeper.LargestBtcReOrg(h.Ctx, &types.QueryLargestBtcReOrgRequest{})
	h.NoError(err)
	require.Equal(t, respLargestReOrg.BlockDiff, currLargestReorg.BlockDiff)
	require.Equal(t, respLargestReOrg.RollbackFrom.HashHex, rBlockFrom.ToResponse().HashHex)
	require.Equal(t, respLargestReOrg.RollbackTo.HashHex, rBlockTo.ToResponse().HashHex)
}

func createActiveBtcDel(t *testing.T, btcLightclientTipHeight uint32) (*testutil.Helper, *rand.Rand, btcctypes.Params, string) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)

	h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

	// set all parameters
	covenantSKs, _ := h.GenAndApplyParams(r)

	// makes sure of the BTC depth
	btcctParams := btcctypes.DefaultParams()
	btccKeeper.EXPECT().GetParams(gomock.Any()).Return(btcctParams).AnyTimes()

	// generate and insert new finality provider
	_, fpPK, _ := h.CreateFinalityProvider(r)

	// generate and insert new BTC delegation
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)

	btcBlockHeightTxInserted := btcLightclientTipHeight - btcctParams.BtcConfirmationDepth
	stakingTxHash, msgCreateBTCDel, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		[]*btcec.PublicKey{fpPK},
		stakingValue,
		1000,
		0,
		0,
		false,
		false,
		btcBlockHeightTxInserted,
		btcLightclientTipHeight,
	)
	h.NoError(err)

	actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	require.NotNil(t, actualDel)

	msgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, msgCreateBTCDel, actualDel)
	for _, msg := range msgs {
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: btcBlockHeightTxInserted})
		_, err = h.MsgServer.AddCovenantSigs(h.Ctx, msg)
		h.NoError(err)
	}

	// ensure consistency between the msg and the BTC delegation in DB
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: btcBlockHeightTxInserted})
	delResp, err := h.BTCStakingKeeper.BTCDelegation(h.Ctx, &types.QueryBTCDelegationRequest{
		StakingTxHashHex: stakingTxHash,
	})
	h.NoError(err)
	require.Equal(t, types.BTCDelegationStatus_ACTIVE.String(), delResp.BtcDelegation.StatusDesc)

	decodeStakingTxHashBz, err := hex.DecodeString(delResp.BtcDelegation.StakingTxHex)
	h.NoError(err)

	decodeStakingTxHash, err := bbn.NewBTCTxFromBytes(decodeStakingTxHashBz)
	h.NoError(err)

	require.Equal(t, decodeStakingTxHash.TxHash().String(), stakingTxHash)
	require.Equal(t, uint32(1), delResp.BtcDelegation.ParamsVersion)

	return h, r, btcctParams, stakingTxHash
}

func TestRejectActivationThatShouldNotUsePreApprovalFlow(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

	// set all parameters
	covenantSKs, _ := h.GenAndApplyParams(r)

	// generate and insert new finality provider
	_, fpPK, _ := h.CreateFinalityProvider(r)

	// create fresh version of params
	currentParams := h.BTCStakingKeeper.GetParams(h.Ctx)
	// params will be activate at block height 2
	currentParams.BtcActivationHeight++
	// Update new params
	err := h.BTCStakingKeeper.SetParams(h.Ctx, currentParams)
	require.NoError(t, err)

	// generate and insert new BTC delegation
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	stakingTxHash, msgCreateBTCDel, _, headerInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		[]*btcec.PublicKey{fpPK},
		stakingValue,
		1000,
		0,
		0,
		// use the pre-approval flow
		true,
		false,
		// staking tx will be included in BTC block height 1, which is before the activation of the new params
		1,
		// current tip is 10
		10,
	)
	h.NoError(err)
	require.NotNil(t, headerInfo)
	require.NotNil(t, inclusionProof)

	// ensure consistency between the msg and the BTC delegation in DB
	actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	require.NotNil(t, actualDel)

	msgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, msgCreateBTCDel, actualDel)
	for _, msg := range msgs {
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: 10})
		_, err = h.MsgServer.AddCovenantSigs(h.Ctx, msg)
		h.NoError(err)
	}

	// get updated delegation
	actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	require.NotNil(t, actualDel)

	tipHeight := uint32(1)
	covenantQuorum := h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum

	status := actualDel.GetStatus(tipHeight, covenantQuorum)
	require.Equal(t, types.BTCDelegationStatus_VERIFIED, status)

	msg := &types.MsgAddBTCDelegationInclusionProof{
		StakingTxHash:           stakingTxHash,
		StakingTxInclusionProof: inclusionProof,
	}

	// mock BTC header that includes the staking tx
	h.BTCLightClientKeeper.EXPECT().GetHeaderByHash(gomock.Eq(h.Ctx), gomock.Eq(headerInfo.Header.Hash())).Return(headerInfo, nil).AnyTimes()
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: 30})
	// Call the AddBTCDelegationInclusionProof handler
	_, err = h.MsgServer.AddBTCDelegationInclusionProof(h.Ctx, msg)
	h.Error(err)
	require.ErrorAs(t, err, &types.ErrStakingTxIncludedTooEarly)
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
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		usePreApproval := datagen.OneInN(r, 2)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)

		var stakingTxHash string
		var msgCreateBTCDel *types.MsgCreateBTCDelegation

		stakingTxHash, msgCreateBTCDel, _, _, _, _, err = h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK},
			stakingValue,
			1000,
			0,
			0,
			usePreApproval,
			false,
			10,
			30,
		)
		h.NoError(err)

		// ensure consistency between the msg and the BTC delegation in DB
		actualDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		// delegation is not activated by covenant yet
		require.False(h.T(), actualDel.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))

		msgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, msgCreateBTCDel, actualDel)
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
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
		require.True(h.T(), actualDel.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))
		require.True(h.T(), actualDel.BtcUndelegation.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))

		tipHeight := uint32(30)
		covenantQuorum := h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum
		status := actualDel.GetStatus(tipHeight, covenantQuorum)
		votingPower := actualDel.VotingPower(tipHeight, covenantQuorum)

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
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK},
			stakingValue,
			1000,
			0,
			0,
			true,
			false,
			10,
			10,
		)
		h.NoError(err)

		// add covenant signatures to this BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)

		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)

		// ensure the BTC delegation is now verified and does not have voting power
		tipHeight := uint32(10)

		covenantQuorum := h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum
		status := actualDel.GetStatus(tipHeight, covenantQuorum)
		votingPower := actualDel.VotingPower(tipHeight, covenantQuorum)

		require.Equal(t, status, types.BTCDelegationStatus_VERIFIED)
		require.Zero(t, votingPower)

		// activate the BTC delegation, such that the BTC delegation becomes active
		// and has voting power
		newTipHeight := uint32(30)
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, newTipHeight)

		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		status = actualDel.GetStatus(tipHeight, covenantQuorum)
		votingPower = actualDel.VotingPower(tipHeight, covenantQuorum)

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
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTime := uint16(1000)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, unbondingInfo, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK},
			stakingValue,
			stakingTime,
			0,
			0,
			true,
			false,
			10,
			10,
		)
		h.NoError(err)

		// add covenant signatures to this BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
		// activate the BTC delegation
		btcTip := uint32(30)
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, btcTip)

		// ensure the BTC delegation is bonded right now
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		status := actualDel.GetStatus(btcTip, bsParams.CovenantQuorum)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

		unbondingTx := actualDel.MustGetUnbondingTx()
		stakingTx := actualDel.MustGetStakingTx()

		serializedUnbondingTxWithWitness, _ := datagen.AddWitnessToUnbondingTx(
			t,
			stakingTx.TxOut[0],
			delSK,
			covenantSKs,
			bsParams.CovenantQuorum,
			[]*btcec.PublicKey{fpPK},
			stakingTime,
			stakingValue,
			unbondingTx,
			h.Net,
		)

		msg := &types.MsgBTCUndelegate{
			Signer:                        datagen.GenRandomAccount().Address,
			StakingTxHash:                 stakingTxHash,
			StakeSpendingTx:               serializedUnbondingTxWithWitness,
			StakeSpendingTxInclusionProof: unbondingInfo.UnbondingTxInclusionProof,
			FundingTransactions: [][]byte{
				actualDel.StakingTx,
			},
		}

		// ensure the system does not panick due to a bogus unbonding msg
		bogusMsg := *msg
		bogusMsg.StakingTxHash = datagen.GenRandomBtcdHash(r).String()
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
		_, err = h.MsgServer.BTCUndelegate(h.Ctx, &bogusMsg)
		h.Error(err)

		// unbond
		_, err = h.MsgServer.BTCUndelegate(h.Ctx, msg)
		h.NoError(err)

		// ensure the BTC delegation is unbonded
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		status = actualDel.GetStatus(btcTip, bsParams.CovenantQuorum)
		require.Equal(t, types.BTCDelegationStatus_UNBONDED, status)
	})
}

func FuzzBTCUndelegateExpired(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// mock BTC light client and BTC checkpoint modules
		btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
		btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)

		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		// generate and insert new finality provider
		_, fpPK, _ := h.CreateFinalityProvider(r)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, unbondingInfo, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK},
			stakingValue,
			1000,
			0,
			0,
			true,
			false,
			10,
			10,
		)
		h.NoError(err)

		// add covenant signatures to this BTC delegation
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
		// activate the BTC delegation
		btcTip := uint32(30)
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, btcTip)

		// ensure the BTC delegation is bonded right now
		actualDel, err = h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
		h.NoError(err)
		status := actualDel.GetStatus(btcTip, bsParams.CovenantQuorum)
		require.Equal(t, types.BTCDelegationStatus_ACTIVE, status)

		msg := &types.MsgBTCUndelegate{
			Signer:                        datagen.GenRandomAccount().Address,
			StakingTxHash:                 stakingTxHash,
			StakeSpendingTx:               actualDel.BtcUndelegation.UnbondingTx,
			StakeSpendingTxInclusionProof: unbondingInfo.UnbondingTxInclusionProof,
		}

		// expires the delegation
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 2000}).AnyTimes()
		_, err = h.MsgServer.BTCUndelegate(h.Ctx, msg)
		require.EqualError(t, err, types.ErrInvalidBTCUndelegateReq.Wrap("cannot unbond an unbonded BTC delegation").Error())
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
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)
		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		// generate and insert new finality provider
		fpSK, fpPK, _ := h.CreateFinalityProvider(r)
		fpBtcPk := bbn.NewBIP340PubKeyFromBTCPK(fpPK)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK},
			stakingValue,
			1000,
			0,
			0,
			true,
			false,
			10,
			10,
		)
		h.NoError(err)

		// add covenant signatures to this BTC delegation
		// so that the BTC delegation becomes bonded
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
		// activate the BTC delegation
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)

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
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()
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
		h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

		// set all parameters
		covenantSKs, _ := h.GenAndApplyParams(r)
		bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

		// generate and insert new finality provider
		fpSK, fpPK, _ := h.CreateFinalityProvider(r)
		fpBtcPk := bbn.NewBIP340PubKeyFromBTCPK(fpPK)

		// generate and insert new BTC delegation
		stakingValue := int64(2 * 10e8)
		delSK, _, err := datagen.GenRandomBTCKeyPair(r)
		h.NoError(err)
		stakingTxHash, msgCreateBTCDel, actualDel, btcHeaderInfo, inclusionProof, _, err := h.CreateDelegationWithBtcBlockHeight(
			r,
			delSK,
			[]*btcec.PublicKey{fpPK},
			stakingValue,
			1000,
			0,
			0,
			true,
			false,
			10,
			10,
		)
		h.NoError(err)

		// add covenant signatures to this BTC delegation
		// so that the BTC delegation becomes bonded
		h.CreateCovenantSigs(r, covenantSKs, msgCreateBTCDel, actualDel, 10)
		// activate the BTC delegation
		h.AddInclusionProof(stakingTxHash, btcHeaderInfo, inclusionProof, 30)
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
		decKey, err := asig.NewDecryptionKeyFromBTCSK(fpSK)
		h.NoError(err)
		covSchnorrSig, err := covASig.Decrypt(decKey)
		require.NoError(t, err)
		decryptedCovenantSig := bbn.NewBIP340SignatureFromBTCSig(covSchnorrSig)

		// recover the fpSK by using adaptor signature and decrypted Schnorr signature
		recoveredFPDecKey, err := covASig.Extract(decryptedCovenantSig.MustToBTCSig())
		require.NoError(t, err)
		recoveredFPSK := recoveredFPDecKey.ToBTCSK()
		// ensure the recovered finality provider SK is same as the real one
		require.Equal(t, fpSK.Serialize(), recoveredFPSK.Serialize())

		// submit evidence of selective slashing
		msg := &types.MsgSelectiveSlashingEvidence{
			Signer:           datagen.GenRandomAccount().Address,
			StakingTxHash:    actualDel.MustGetStakingTxHash().String(),
			RecoveredFpBtcSk: recoveredFPSK.Serialize(),
		}
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: 30}).AnyTimes()

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
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

	// set covenant PK to params
	_, covenantPKs := h.GenAndApplyParams(r)
	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

	unbondingTime := bsParams.UnbondingTimeBlocks

	slashingChangeLockTime := uint16(unbondingTime)

	// We only generate a finality provider, but not insert it into KVStore. So later
	// insertion of delegation should fail.

	fp, err := datagen.GenRandomFinalityProvider(r, h.FpPopContext(), "")
	require.NoError(t, err)

	/*
		generate and insert valid new BTC delegation
	*/
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	stakingTimeBlocks := bsParams.MinStakingTimeBlocks
	stakingValue := int64(2 * 10e8)
	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r,
		t,
		h.Net,
		delSK,
		[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
		covenantPKs,
		bsParams.CovenantQuorum,
		uint16(stakingTimeBlocks),
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
	pop, err := datagen.NewPoPBTC(h.StakerPopContext(), stakerAddr, delSK)
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
	unbondingValue := stakingValue - datagen.UnbondingTxFee // TODO: parameterise fee
	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		h.Net,
		delSK,
		[]*btcec.PublicKey{fp.BtcPk.MustToBTCPK()},
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
	msgCreateBTCDel := &types.MsgCreateBTCDelegation{
		StakerAddr:                    stakerAddr.String(),
		FpBtcPkList:                   []bbn.BIP340PubKey{*fp.BtcPk},
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(delSK.PubKey()),
		Pop:                           pop,
		StakingTime:                   stakingTimeBlocks,
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		StakingTxInclusionProof:       txInclusionProof,
		SlashingTx:                    testStakingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingTx:                   unbondingTx,
		UnbondingTime:                 unbondingTime,
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           testUnbondingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delUnbondingSlashingSig,
	}

	_, err = h.MsgServer.CreateBTCDelegation(h.Ctx, msgCreateBTCDel)
	require.Error(t, err)
	require.True(t, errors.Is(err, types.ErrFpNotFound))

	AddFinalityProvider(t, h.Ctx, *h.BTCStakingKeeper, fp)
	inclusionHeight := uint32(100)
	inclusionHeader := &btclctypes.BTCHeaderInfo{
		Header: &btcHeader,
		Height: inclusionHeight,
	}
	tipHeight := 150
	mockTipHeaderInfo := &btclctypes.BTCHeaderInfo{Height: uint32(tipHeight)}
	btclcKeeper.EXPECT().GetHeaderByHash(gomock.Any(), btcHeader.Hash()).Return(inclusionHeader, nil).Times(1)
	btclcKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(mockTipHeaderInfo).Times(1)
	_, err = h.MsgServer.CreateBTCDelegation(h.Ctx, msgCreateBTCDel)
	require.NoError(t, err)
}

func TestCorrectUnbondingTimeInDelegation(t *testing.T) {
	tests := []struct {
		name                      string
		finalizationTimeout       uint32
		unbondingTimeInParams     uint32
		unbondingTimeInDelegation uint16
		err                       error
	}{
		{
			name:                      "successful delegation if unbonding time in delegation is equal to unbonding time in params",
			unbondingTimeInDelegation: 101,
			unbondingTimeInParams:     101,
			finalizationTimeout:       100,
			err:                       nil,
		},
		{
			name:                      "invalid delegation if unbonding time is different from unbonding time in params",
			unbondingTimeInDelegation: 102,
			unbondingTimeInParams:     101,
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
			h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

			// set all parameters
			_, _ = h.GenAndApplyCustomParams(r, tt.finalizationTimeout, tt.unbondingTimeInParams, 0)

			// generate and insert new finality provider
			_, fpPK, _ := h.CreateFinalityProvider(r)

			// generate and insert new BTC delegation
			stakingValue := int64(2 * 10e8)
			delSK, _, err := datagen.GenRandomBTCKeyPair(r)
			h.NoError(err)
			stakingTxHash, _, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
				r,
				delSK,
				[]*btcec.PublicKey{fpPK},
				stakingValue,
				1000,
				stakingValue-1000,
				tt.unbondingTimeInDelegation,
				true,
				false,
				10,
				30,
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

func TestAllowList(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// mock BTC light client and BTC checkpoint modules
	btclcKeeper := types.NewMockBTCLightClientKeeper(ctrl)
	btccKeeper := types.NewMockBtcCheckpointKeeper(ctrl)
	h := testutil.NewHelper(t, btclcKeeper, btccKeeper)

	allowListExpirationHeight := uint64(10)
	// set all parameters, use the allow list
	h.GenAndApplyCustomParams(r, 100, 0, allowListExpirationHeight)

	// generate and insert new finality provider
	_, fpPK, _ := h.CreateFinalityProvider(r)

	usePreApproval := datagen.OneInN(r, 2)

	// generate and insert new BTC delegation
	stakingValue := int64(2 * 10e8)
	delSK, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	_, msgCreateBTCDel, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK,
		[]*btcec.PublicKey{fpPK},
		stakingValue,
		1000,
		0,
		0,
		usePreApproval,
		// add delegation to the allow list, it should succeed
		true,
		10,
		30,
	)
	h.NoError(err)
	require.NotNil(t, msgCreateBTCDel)

	delSK1, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	_, msgCreateBTCDel1, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK1,
		[]*btcec.PublicKey{fpPK},
		stakingValue,
		1000,
		0,
		0,
		usePreApproval,
		// do not add delegation to the allow list, it should fail
		false,
		10,
		30,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, types.ErrInvalidStakingTx)
	require.Nil(t, msgCreateBTCDel1)

	// move forward in the block height, allow list should be expired
	h.Ctx = h.Ctx.WithBlockHeight(int64(allowListExpirationHeight))
	delSK2, _, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	_, msgCreateBTCDel2, _, _, _, _, err := h.CreateDelegationWithBtcBlockHeight(
		r,
		delSK2,
		[]*btcec.PublicKey{fpPK},
		stakingValue,
		1000,
		0,
		0,
		usePreApproval,
		// do not add delegation to the allow list, it should succeed as allow list is expired
		false,
		10,
		30,
	)
	h.NoError(err)
	require.NotNil(t, msgCreateBTCDel2)
}

func createNDelegationsForFinalityProvider(
	r *rand.Rand,
	t *testing.T,
	signingContext string,
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
			signingContext,
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

type ExpectedProviderData struct {
	numDelegations int32
	stakingValue   int32
}

func FuzzDeterminismBtcstakingBeginBlocker(f *testing.F) {
	// less seeds than usual as this is pretty long running test
	datagen.AddRandomSeedsToFuzzer(f, 5)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		valSet, privSigner, err := datagen.GenesisValidatorSetWithPrivSigner(2)
		require.NoError(t, err)

		var expectedProviderData = make(map[string]*ExpectedProviderData)

		// Create two test apps from the same set of validators
		h := testhelper.NewHelperWithValSet(t, valSet, privSigner)
		h1 := testhelper.NewHelperWithValSet(t, valSet, privSigner)
		// app hash should be same at the beginning
		appHash1 := hex.EncodeToString(h.Ctx.BlockHeader().AppHash)
		appHash2 := hex.EncodeToString(h1.Ctx.BlockHeader().AppHash)
		require.Equal(t, appHash1, appHash2)

		// Execute block for both apps
		h.Ctx, err = h.ApplyEmptyBlockWithVoteExtension(r)
		require.NoError(t, err)
		h1.Ctx, err = h1.ApplyEmptyBlockWithVoteExtension(r)
		require.NoError(t, err)
		// Given that there is no transactions and the data in db is the same
		// app hash produced by both apps should be the same
		appHash1 = hex.EncodeToString(h.Ctx.BlockHeader().AppHash)
		appHash2 = hex.EncodeToString(h1.Ctx.BlockHeader().AppHash)
		require.Equal(t, appHash1, appHash2)

		// Default params are the same in both apps
		stakingParams := h.App.BTCStakingKeeper.GetParams(h.Ctx)
		covQuorum := stakingParams.CovenantQuorum
		maxFinalityProviders := int32(h.App.FinalityKeeper.GetParams(h.Ctx).MaxActiveFinalityProviders)

		// Number of finality providers from 10 to maxFinalityProviders + 10
		numFinalityProviders := int(r.Int31n(maxFinalityProviders) + 10)

		fps := datagen.CreateNFinalityProviders(r, t, h.FpPopContext(), "", numFinalityProviders)

		// Fill the database of both apps with the same finality providers and delegations
		for _, fp := range fps {
			h.AddFinalityProvider(fp)
			h1.AddFinalityProvider(fp)
		}

		for _, fp := range fps {
			// each finality provider has different amount of delegations with different amount
			stakingValue := r.Int31n(200000) + 10000
			numDelegations := r.Int31n(10)

			if numDelegations > 0 {
				expectedProviderData[fp.BtcPk.MarshalHex()] = &ExpectedProviderData{
					numDelegations: numDelegations,
					stakingValue:   stakingValue,
				}
			}

			delegations := createNDelegationsForFinalityProvider(
				r,
				t,
				h.StakerPopContext(),
				fp.BtcPk.MustToBTCPK(),
				int64(stakingValue),
				int(numDelegations),
				covQuorum,
			)

			for _, del := range delegations {
				h.AddDelegation(del)
				h1.AddDelegation(del)
			}
		}

		// Execute block for both apps
		h.Ctx, err = h.ApplyEmptyBlockWithVoteExtension(r)
		require.NoError(t, err)
		h1.Ctx, err = h1.ApplyEmptyBlockWithVoteExtension(r)
		require.NoError(t, err)
		// Given that there is no transactions and the data in db is the same
		// app hash produced by both apps should be the same
		appHash1 = hex.EncodeToString(h.Ctx.BlockHeader().AppHash)
		appHash2 = hex.EncodeToString(h1.Ctx.BlockHeader().AppHash)
		require.Equal(t, appHash1, appHash2)
	})
}
