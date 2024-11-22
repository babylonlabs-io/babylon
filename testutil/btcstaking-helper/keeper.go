package testutil

import (
	"math/rand"
	"testing"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	dbm "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/keeper"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
	epochingtypes "github.com/babylonlabs-io/babylon/x/epoching/types"
	fkeeper "github.com/babylonlabs-io/babylon/x/finality/keeper"
	ftypes "github.com/babylonlabs-io/babylon/x/finality/types"
)

var (
	btcTipHeight     = uint32(30)
	timestampedEpoch = uint64(10)
)

type Helper struct {
	t testing.TB

	Ctx              sdk.Context
	BTCStakingKeeper *keeper.Keeper
	MsgServer        types.MsgServer

	FinalityKeeper *fkeeper.Keeper
	FMsgServer     ftypes.MsgServer

	BTCLightClientKeeper *types.MockBTCLightClientKeeper
	BTCCheckpointKeeper  *types.MockBtcCheckpointKeeper
	CheckpointingKeeper  *ftypes.MockCheckpointingKeeper
	Net                  *chaincfg.Params
}

type UnbondingTxInfo struct {
	UnbondingTxInclusionProof *types.InclusionProof
	UnbondingHeaderInfo       *btclctypes.BTCHeaderInfo
}

func NewHelper(
	t testing.TB,
	btclcKeeper *types.MockBTCLightClientKeeper,
	btccKeeper *types.MockBtcCheckpointKeeper,
) *Helper {
	ctrl := gomock.NewController(t)

	// mock refundable messages
	iKeeper := ftypes.NewMockIncentiveKeeper(ctrl)
	iKeeper.EXPECT().IndexRefundableMsg(gomock.Any(), gomock.Any()).AnyTimes()

	ckptKeeper := ftypes.NewMockCheckpointingKeeper(ctrl)
	ckptKeeper.EXPECT().GetLastFinalizedEpoch(gomock.Any()).Return(timestampedEpoch).AnyTimes()

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	k, _ := keepertest.BTCStakingKeeperWithStore(t, db, stateStore, btclcKeeper, btccKeeper, iKeeper)
	msgSrvr := keeper.NewMsgServerImpl(*k)

	fk, ctx := keepertest.FinalityKeeperWithStore(t, db, stateStore, k, iKeeper, ckptKeeper)
	fMsgSrvr := fkeeper.NewMsgServerImpl(*fk)

	// set all parameters
	err := k.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)
	err = fk.SetParams(ctx, ftypes.DefaultParams())
	require.NoError(t, err)

	ctx = ctx.WithHeaderInfo(header.Info{Height: 1}).WithBlockHeight(1)

	return &Helper{
		t:   t,
		Ctx: ctx,

		BTCStakingKeeper: k,
		MsgServer:        msgSrvr,

		FinalityKeeper: fk,
		FMsgServer:     fMsgSrvr,

		BTCLightClientKeeper: btclcKeeper,
		BTCCheckpointKeeper:  btccKeeper,
		CheckpointingKeeper:  ckptKeeper,
		Net:                  &chaincfg.SimNetParams,
	}
}

func (h *Helper) T() testing.TB {
	return h.t
}

func (h *Helper) NoError(err error) {
	require.NoError(h.t, err)
}

func (h *Helper) Error(err error, msgAndArgs ...any) {
	require.Error(h.t, err, msgAndArgs...)
}

func (h *Helper) BeginBlocker() {
	err := h.BTCStakingKeeper.BeginBlocker(h.Ctx)
	h.NoError(err)
	err = h.FinalityKeeper.BeginBlocker(h.Ctx)
	h.NoError(err)
}

func (h *Helper) GenAndApplyParams(r *rand.Rand) ([]*btcec.PrivateKey, []*btcec.PublicKey) {
	// ensure that minUnbondingTime is larger than finalizationTimeout
	return h.GenAndApplyCustomParams(r, 100, 200, 0)
}

func (h *Helper) SetCtxHeight(height uint64) {
	h.Ctx = datagen.WithCtxHeight(h.Ctx, height)
}

func (h *Helper) GenAndApplyCustomParams(
	r *rand.Rand,
	finalizationTimeout uint32,
	minUnbondingTime uint32,
	allowListExpirationHeight uint64,
) ([]*btcec.PrivateKey, []*btcec.PublicKey) {
	// mock base header
	baseHeader := btclctypes.SimnetGenesisBlock()
	h.BTCLightClientKeeper.EXPECT().GetBaseBTCHeader(gomock.Any()).Return(&baseHeader).AnyTimes()

	params := btcctypes.DefaultParams()
	params.CheckpointFinalizationTimeout = finalizationTimeout

	h.BTCCheckpointKeeper.EXPECT().GetParams(gomock.Any()).Return(params).AnyTimes()

	// randomise covenant committee
	covenantSKs, covenantPKs, err := datagen.GenRandomBTCKeyPairs(r, 5)
	h.NoError(err)
	slashingAddress, err := datagen.GenRandomBTCAddress(r, h.Net)
	h.NoError(err)
	slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
	h.NoError(err)
	err = h.BTCStakingKeeper.SetParams(h.Ctx, types.Params{
		CovenantPks:               bbn.NewBIP340PKsFromBTCPKs(covenantPKs),
		CovenantQuorum:            3,
		MinStakingValueSat:        1000,
		MaxStakingValueSat:        int64(4 * 10e8),
		MinStakingTimeBlocks:      400,
		MaxStakingTimeBlocks:      10000,
		SlashingPkScript:          slashingPkScript,
		MinSlashingTxFeeSat:       10,
		MinCommissionRate:         sdkmath.LegacyMustNewDecFromStr("0.01"),
		SlashingRate:              sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2),
		MinUnbondingTimeBlocks:    minUnbondingTime,
		UnbondingFeeSat:           1000,
		AllowListExpirationHeight: allowListExpirationHeight,
	})
	h.NoError(err)
	return covenantSKs, covenantPKs
}

func CreateFinalityProvider(r *rand.Rand, t *testing.T) *types.FinalityProvider {
	fpSK, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, fpSK)
	require.NoError(t, err)

	return &types.FinalityProvider{
		Description: fp.Description,
		Commission:  fp.Commission,
		Addr:        fp.Addr,
		BtcPk:       fp.BtcPk,
		Pop:         fp.Pop,
	}
}

func (h *Helper) CreateFinalityProvider(r *rand.Rand) (*btcec.PrivateKey, *btcec.PublicKey, *types.FinalityProvider) {
	fpSK, fpPK, err := datagen.GenRandomBTCKeyPair(r)
	h.NoError(err)
	fp, err := datagen.GenRandomFinalityProviderWithBTCSK(r, fpSK)
	h.NoError(err)
	msgNewFp := types.MsgCreateFinalityProvider{
		Addr:        fp.Addr,
		Description: fp.Description,
		Commission:  fp.Commission,
		BtcPk:       fp.BtcPk,
		Pop:         fp.Pop,
	}

	_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, &msgNewFp)
	h.NoError(err)
	return fpSK, fpPK, fp
}

func (h *Helper) CreateDelegation(
	r *rand.Rand,
	delSK *btcec.PrivateKey,
	fpPK *btcec.PublicKey,
	changeAddress string,
	stakingValue int64,
	stakingTime uint16,
	unbondingValue int64,
	unbondingTime uint16,
	usePreApproval bool,
	addToAllowList bool,
) (string, *types.MsgCreateBTCDelegation, *types.BTCDelegation, *btclctypes.BTCHeaderInfo, *types.InclusionProof, *UnbondingTxInfo, error) {
	stakingTimeBlocks := stakingTime
	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)
	bcParams := h.BTCCheckpointKeeper.GetParams(h.Ctx)
	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(bsParams.CovenantPks)
	h.NoError(err)

	// if not set, use default values for unbonding value and time
	defaultUnbondingValue := stakingValue - 1000
	if unbondingValue == 0 {
		unbondingValue = defaultUnbondingValue
	}
	defaultUnbondingTime := bsParams.MinUnbondingTimeBlocks
	if unbondingTime == 0 {
		unbondingTime = uint16(defaultUnbondingTime)
	}

	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r,
		h.t,
		h.Net,
		delSK,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		bsParams.CovenantQuorum,
		stakingTimeBlocks,
		stakingValue,
		bsParams.SlashingPkScript,
		bsParams.SlashingRate,
		unbondingTime,
	)
	h.NoError(err)
	stakingTxHash := testStakingInfo.StakingTx.TxHash().String()

	// random signer
	staker := sdk.MustAccAddressFromBech32(datagen.GenRandomAccount().Address)

	// PoP
	pop, err := types.NewPoPBTC(staker, delSK)
	h.NoError(err)
	// generate staking tx info
	prevBlock, _ := datagen.GenRandomBtcdBlock(r, 0, nil)
	btcHeaderWithProof := datagen.CreateBlockWithTransaction(r, &prevBlock.Header, testStakingInfo.StakingTx)
	btcHeader := btcHeaderWithProof.HeaderBytes
	btcHeaderInfo := &btclctypes.BTCHeaderInfo{Header: &btcHeader, Height: 10}
	serializedStakingTx, err := bbn.SerializeBTCTx(testStakingInfo.StakingTx)
	h.NoError(err)

	txInclusionProof := types.NewInclusionProof(&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()}, btcHeaderWithProof.SpvProof.MerkleNodes)

	// mock for testing k-deep stuff
	h.BTCLightClientKeeper.EXPECT().GetHeaderByHash(gomock.Eq(h.Ctx), gomock.Eq(btcHeader.Hash())).Return(btcHeaderInfo).AnyTimes()
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: btcTipHeight}).AnyTimes()

	slashingSpendInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	h.NoError(err)

	// generate proper delegator sig
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		testStakingInfo.StakingTx,
		0,
		slashingSpendInfo.GetPkScriptPath(),
		delSK,
	)
	h.NoError(err)

	stakerPk := delSK.PubKey()
	stPk := bbn.NewBIP340PubKeyFromBTCPK(stakerPk)

	/*
		logics related to on-demand unbonding
	*/
	stkTxHash := testStakingInfo.StakingTx.TxHash()
	stkOutputIdx := uint32(0)

	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		h.t,
		h.Net,
		delSK,
		[]*btcec.PublicKey{fpPK},
		covPKs,
		bsParams.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, stkOutputIdx),
		unbondingTime,
		unbondingValue,
		bsParams.SlashingPkScript,
		bsParams.SlashingRate,
		unbondingTime,
	)
	h.NoError(err)

	delSlashingTxSig, err := testUnbondingInfo.GenDelSlashingTxSig(delSK)
	h.NoError(err)

	serializedUnbondingTx, err := bbn.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	h.NoError(err)

	prevBlockForUnbonding, _ := datagen.GenRandomBtcdBlock(r, 0, nil)
	btcUnbondingHeaderWithProof := datagen.CreateBlockWithTransaction(r, &prevBlockForUnbonding.Header, testUnbondingInfo.UnbondingTx)
	btcUnbondingHeader := btcUnbondingHeaderWithProof.HeaderBytes
	btcUnbondingHeaderInfo := &btclctypes.BTCHeaderInfo{Header: &btcUnbondingHeader, Height: 11}
	unbondingTxInclusionProof := types.NewInclusionProof(
		&btcctypes.TransactionKey{Index: 1, Hash: btcUnbondingHeader.Hash()},
		btcUnbondingHeaderWithProof.SpvProof.MerkleNodes,
	)
	h.BTCLightClientKeeper.EXPECT().GetHeaderByHash(gomock.Eq(h.Ctx), gomock.Eq(btcUnbondingHeader.Hash())).Return(btcUnbondingHeaderInfo).AnyTimes()

	// all good, construct and send MsgCreateBTCDelegation message
	fpBTCPK := bbn.NewBIP340PubKeyFromBTCPK(fpPK)
	msgCreateBTCDel := &types.MsgCreateBTCDelegation{
		StakerAddr:                    staker.String(),
		BtcPk:                         stPk,
		FpBtcPkList:                   []bbn.BIP340PubKey{*fpBTCPK},
		Pop:                           pop,
		StakingTime:                   uint32(stakingTimeBlocks),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		SlashingTx:                    testStakingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingTx:                   serializedUnbondingTx,
		UnbondingTime:                 uint32(unbondingTime),
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           testUnbondingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delSlashingTxSig,
	}

	if !usePreApproval {
		msgCreateBTCDel.StakingTxInclusionProof = txInclusionProof
	}

	if addToAllowList {
		h.BTCStakingKeeper.IndexAllowedStakingTransaction(h.Ctx, &stkTxHash)
	}

	_, err = h.MsgServer.CreateBTCDelegation(h.Ctx, msgCreateBTCDel)
	if err != nil {
		return "", nil, nil, nil, nil, nil, err
	}

	stakingMsgTx, err := bbn.NewBTCTxFromBytes(msgCreateBTCDel.StakingTx)
	h.NoError(err)
	btcDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingMsgTx.TxHash().String())
	h.NoError(err)

	// ensure the delegation is still pending
	require.Equal(h.t, btcDel.GetStatus(btcTipHeight, bcParams.CheckpointFinalizationTimeout, bsParams.CovenantQuorum), types.BTCDelegationStatus_PENDING)

	if usePreApproval {
		// the BTC delegation does not have inclusion proof
		require.False(h.t, btcDel.HasInclusionProof())
	} else {
		// the BTC delegation has inclusion proof
		require.True(h.t, btcDel.HasInclusionProof())
	}

	return stakingTxHash, msgCreateBTCDel, btcDel, btcHeaderInfo, txInclusionProof, &UnbondingTxInfo{
		UnbondingTxInclusionProof: unbondingTxInclusionProof,
		UnbondingHeaderInfo:       btcUnbondingHeaderInfo,
	}, nil
}

func (h *Helper) GenerateCovenantSignaturesMessages(
	r *rand.Rand,
	covenantSKs []*btcec.PrivateKey,
	msgCreateBTCDel *types.MsgCreateBTCDelegation,
	del *types.BTCDelegation,
) []*types.MsgAddCovenantSigs {
	stakingTx, err := bbn.NewBTCTxFromBytes(del.StakingTx)
	h.NoError(err)
	stakingTxHash := stakingTx.TxHash().String()

	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

	vPKs, err := bbn.NewBTCPKsFromBIP340PKs(del.FpBtcPkList)
	h.NoError(err)

	stakingInfo, err := del.GetStakingInfo(&bsParams, h.Net)
	h.NoError(err)

	unbondingPathInfo, err := stakingInfo.UnbondingPathSpendInfo()
	h.NoError(err)
	slashingPathInfo, err := stakingInfo.SlashingPathSpendInfo()
	h.NoError(err)

	// generate all covenant signatures from all covenant members
	covenantSlashingTxSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs,
		vPKs,
		stakingTx,
		slashingPathInfo.GetPkScriptPath(),
		msgCreateBTCDel.SlashingTx,
	)
	h.NoError(err)

	/*
		Logics about on-demand unbonding
	*/

	// slash unbonding tx spends unbonding tx
	unbondingTx, err := bbn.NewBTCTxFromBytes(del.BtcUndelegation.UnbondingTx)
	h.NoError(err)
	unbondingInfo, err := del.GetUnbondingInfo(&bsParams, h.Net)
	h.NoError(err)
	unbondingSlashingPathInfo, err := unbondingInfo.SlashingPathSpendInfo()
	h.NoError(err)

	// generate all covenant signatures from all covenant members
	covenantUnbondingSlashingTxSigs, err := datagen.GenCovenantAdaptorSigs(
		covenantSKs,
		vPKs,
		unbondingTx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		del.BtcUndelegation.SlashingTx,
	)
	h.NoError(err)

	// each covenant member submits signatures
	covUnbondingSigs, err := datagen.GenCovenantUnbondingSigs(covenantSKs, stakingTx, del.StakingOutputIdx, unbondingPathInfo.GetPkScriptPath(), unbondingTx)
	h.NoError(err)

	msgs := make([]*types.MsgAddCovenantSigs, len(bsParams.CovenantPks))

	for i := 0; i < len(bsParams.CovenantPks); i++ {
		msgAddCovenantSig := &types.MsgAddCovenantSigs{
			Signer:                  msgCreateBTCDel.StakerAddr,
			Pk:                      covenantSlashingTxSigs[i].CovPk,
			StakingTxHash:           stakingTxHash,
			SlashingTxSigs:          covenantSlashingTxSigs[i].AdaptorSigs,
			UnbondingTxSig:          bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
			SlashingUnbondingTxSigs: covenantUnbondingSlashingTxSigs[i].AdaptorSigs,
		}
		msgs[i] = msgAddCovenantSig
	}
	return msgs
}

func (h *Helper) CreateCovenantSigs(
	r *rand.Rand,
	covenantSKs []*btcec.PrivateKey,
	msgCreateBTCDel *types.MsgCreateBTCDelegation,
	del *types.BTCDelegation,
) {
	bcParams := h.BTCCheckpointKeeper.GetParams(h.Ctx)
	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

	stakingTx, err := bbn.NewBTCTxFromBytes(del.StakingTx)
	h.NoError(err)
	stakingTxHash := stakingTx.TxHash().String()

	covenantMsgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, msgCreateBTCDel, del)
	for _, m := range covenantMsgs {
		msgCopy := m
		_, err := h.MsgServer.AddCovenantSigs(h.Ctx, msgCopy)
		h.NoError(err)
	}
	/*
		ensure covenant sig is added successfully
	*/
	actualDelWithCovenantSigs, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	require.Equal(h.t, len(actualDelWithCovenantSigs.CovenantSigs), len(covenantMsgs))
	require.True(h.t, actualDelWithCovenantSigs.HasCovenantQuorums(h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum))

	require.NotNil(h.t, actualDelWithCovenantSigs.BtcUndelegation)
	require.NotNil(h.t, actualDelWithCovenantSigs.BtcUndelegation.CovenantSlashingSigs)
	require.NotNil(h.t, actualDelWithCovenantSigs.BtcUndelegation.CovenantUnbondingSigList)
	require.Len(h.t, actualDelWithCovenantSigs.BtcUndelegation.CovenantUnbondingSigList, len(covenantMsgs))
	require.Len(h.t, actualDelWithCovenantSigs.BtcUndelegation.CovenantSlashingSigs, len(covenantMsgs))
	require.Len(h.t, actualDelWithCovenantSigs.BtcUndelegation.CovenantSlashingSigs[0].AdaptorSigs, 1)

	// ensure the BTC delegation is verified (if using pre-approval flow) or active
	status := actualDelWithCovenantSigs.GetStatus(btcTipHeight, bcParams.CheckpointFinalizationTimeout, bsParams.CovenantQuorum)
	if msgCreateBTCDel.StakingTxInclusionProof != nil {
		// not pre-approval flow, the BTC delegation should be active
		require.Equal(h.t, status, types.BTCDelegationStatus_ACTIVE)
	} else {
		// pre-approval flow, the BTC delegation should be verified
		require.Equal(h.t, status, types.BTCDelegationStatus_VERIFIED)
	}
}

func (h *Helper) AddInclusionProof(
	stakingTxHash string,
	btcHeader *btclctypes.BTCHeaderInfo,
	proof *types.InclusionProof,
) {
	bcParams := h.BTCCheckpointKeeper.GetParams(h.Ctx)
	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

	// Get the BTC delegation and ensure it's verified
	del, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	status := del.GetStatus(btcTipHeight, bcParams.CheckpointFinalizationTimeout, bsParams.CovenantQuorum)
	require.Equal(h.t, status, types.BTCDelegationStatus_VERIFIED, "the BTC delegation shall be verified")

	// Create the MsgAddBTCDelegationInclusionProof message
	msg := &types.MsgAddBTCDelegationInclusionProof{
		StakingTxHash:           stakingTxHash,
		StakingTxInclusionProof: proof,
	}

	// mock BTC header that includes the staking tx
	h.BTCLightClientKeeper.EXPECT().GetHeaderByHash(gomock.Eq(h.Ctx), gomock.Eq(btcHeader.Header.Hash())).Return(btcHeader).AnyTimes()

	// Call the AddBTCDelegationInclusionProof handler
	_, err = h.MsgServer.AddBTCDelegationInclusionProof(h.Ctx, msg)
	h.NoError(err)

	// Verify that the inclusion proof is added successfully and the BTC delegation
	// has been activated
	updatedDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	status = updatedDel.GetStatus(btcTipHeight, bcParams.CheckpointFinalizationTimeout, bsParams.CovenantQuorum)
	require.Equal(h.t, status, types.BTCDelegationStatus_ACTIVE, "the BTC delegation shall be active")
}

func (h *Helper) CommitPubRandList(
	r *rand.Rand,
	fpSK *btcec.PrivateKey,
	fp *types.FinalityProvider,
	startHeight uint64,
	numPubRand uint64,
	timestamped bool,
) *datagen.RandListInfo {
	randListInfo, msg, err := datagen.GenRandomMsgCommitPubRandList(r, fpSK, startHeight, numPubRand)
	h.NoError(err)

	// if timestamped, use the timestamped epoch, otherwise use the next epoch
	var epoch uint64
	if timestamped {
		epoch = timestampedEpoch
	} else {
		epoch = timestampedEpoch + 1
	}

	h.CheckpointingKeeper.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: epoch}).Times(1)

	_, err = h.FMsgServer.CommitPubRandList(h.Ctx, msg)
	h.NoError(err)

	return randListInfo
}
