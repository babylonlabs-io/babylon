package testutil

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/core/header"
	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"
	storemetrics "cosmossdk.io/store/metrics"
	storetypes "cosmossdk.io/store/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	dbm "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	keepertest "github.com/babylonlabs-io/babylon/v4/testutil/keeper"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcctypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/v4/x/btclightclient/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	costktypes "github.com/babylonlabs-io/babylon/v4/x/costaking/types"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	fkeeper "github.com/babylonlabs-io/babylon/v4/x/finality/keeper"
	ftypes "github.com/babylonlabs-io/babylon/v4/x/finality/types"
)

var (
	btcTipHeight     = uint32(30)
	timestampedEpoch = uint64(10)
)

type IctvKeeperI interface {
	ftypes.IncentiveKeeper
	types.IncentiveKeeper
}

// IctvKeeperK this structure is only test useful
// It wraps two instances of the incentive keeper to create the test suite
type IctvKeeperK struct {
	*ftypes.MockIncentiveKeeper
	MockBtcStk *types.MockIncentiveKeeper
}

func NewMockIctvKeeperK(ctrl *gomock.Controller) *IctvKeeperK {
	ictvFinalK := ftypes.NewMockIncentiveKeeper(ctrl)
	ictvBstkK := types.NewMockIncentiveKeeper(ctrl)

	return &IctvKeeperK{
		MockIncentiveKeeper: ictvFinalK,
		MockBtcStk:          ictvBstkK,
	}
}

type Helper struct {
	t testing.TB

	Ctx              sdk.Context
	BTCStakingKeeper *keeper.Keeper
	MsgServer        types.MsgServer

	FinalityKeeper *fkeeper.Keeper
	FMsgServer     ftypes.MsgServer
	FinalityHooks  ftypes.FinalityHooks

	BTCLightClientKeeper             *types.MockBTCLightClientKeeper
	CheckpointingKeeperForBtcStaking *types.MockBtcCheckpointKeeper
	CheckpointingKeeperForFinality   *ftypes.MockCheckpointingKeeper
	IncentiveKeeper                  types.IncentiveKeeper
	Net                              *chaincfg.Params
}

type UnbondingTxInfo struct {
	UnbondingTxInclusionProof *types.InclusionProof
	UnbondingHeaderInfo       *btclctypes.BTCHeaderInfo
}

func NewHelper(
	t testing.TB,
	btclcKeeper *types.MockBTCLightClientKeeper,
	btccKeeper *types.MockBtcCheckpointKeeper,
	btcStkStoreKey *storetypes.KVStoreKey,
) *Helper {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	// mock refundable messages
	iKeeper := ftypes.NewMockIncentiveKeeper(ctrl)
	iKeeper.EXPECT().IndexRefundableMsg(gomock.Any(), gomock.Any()).AnyTimes()
	iKeeper.EXPECT().AddEventBtcDelegationActivated(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	iKeeper.EXPECT().AddEventBtcDelegationUnbonded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	ckptKeeper := ftypes.NewMockCheckpointingKeeper(ctrl)
	ckptKeeper.EXPECT().GetLastFinalizedEpoch(gomock.Any()).Return(timestampedEpoch).AnyTimes()

	return NewHelperWithStoreAndIncentive(t, db, stateStore, btclcKeeper, btccKeeper, ckptKeeper, iKeeper, btcStkStoreKey, ftypes.NewMockFinalityHooks(ctrl))
}

func (h *Helper) WithBlockHeight(height int64) *Helper {
	h.Ctx = h.Ctx.WithBlockHeight(height)
	h.Ctx = h.Ctx.WithHeaderInfo(header.Info{Height: height, Time: time.Now()})
	return h
}

func NewHelperNoMocksCalls(
	t testing.TB,
	btclcKeeper *types.MockBTCLightClientKeeper,
	btccKeeper *types.MockBtcCheckpointKeeper,
	btcStkStoreKey *storetypes.KVStoreKey,
) *Helper {
	ctrl := gomock.NewController(t)

	iKeeper := ftypes.NewMockIncentiveKeeper(ctrl)
	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	ckptKeeper := ftypes.NewMockCheckpointingKeeper(ctrl)

	return NewHelperWithStoreAndIncentive(t, db, stateStore, btclcKeeper, btccKeeper, ckptKeeper, iKeeper, btcStkStoreKey, ftypes.NewMockFinalityHooks(ctrl))
}

// NewHelperWithIncentiveKeeper creates a new Helper with the given BTCLightClientKeeper and BtcCheckpointKeeper mocks, and an instance of the incentive keeper.
func NewHelperWithIncentiveKeeper(
	t testing.TB,
	btclcKeeper *types.MockBTCLightClientKeeper,
	btccKeeper *types.MockBtcCheckpointKeeper,
) *Helper {
	ctrl := gomock.NewController(t)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	accK := keepertest.AccountKeeper(t, db, stateStore)
	bankK := keepertest.BankKeeper(t, db, stateStore, accK)

	iKeeper, _ := keepertest.IncentiveKeeperWithStore(t, db, stateStore, nil, bankK, accK, nil)

	ckptKeeper := ftypes.NewMockCheckpointingKeeper(ctrl)

	return NewHelperWithStoreAndIncentive(t, db, stateStore, btclcKeeper, btccKeeper, ckptKeeper, iKeeper, nil, ftypes.NewMockFinalityHooks(ctrl))
}

func NewHelperWithBankMock(
	t testing.TB,
	btclcKeeper *types.MockBTCLightClientKeeper,
	btccKeeper *types.MockBtcCheckpointKeeper,
	bankKeeper *costktypes.MockBankKeeper,
	iKeeper types.IncentiveKeeper,
	btcStkStoreKey *storetypes.KVStoreKey,
) *Helper {
	ctrl := gomock.NewController(t)

	db := dbm.NewMemDB()
	stateStore := store.NewCommitMultiStore(db, log.NewTestLogger(t), storemetrics.NewNoOpMetrics())

	ckptKeeper := ftypes.NewMockCheckpointingKeeper(ctrl)

	return NewHelperWithStoreIncentiveAndBank(t, db, stateStore, btclcKeeper, btccKeeper, ckptKeeper, iKeeper, bankKeeper, btcStkStoreKey)
}

func NewHelperWithStoreAndIncentive(
	t testing.TB,
	db dbm.DB,
	stateStore store.CommitMultiStore,
	btclcKeeper *types.MockBTCLightClientKeeper,
	btccKForBtcStaking *types.MockBtcCheckpointKeeper,
	btccKForFinality *ftypes.MockCheckpointingKeeper,
	iKeepereeper ftypes.IncentiveKeeper,
	btcStkStoreKey *storetypes.KVStoreKey,
	finalityHooks ftypes.FinalityHooks,
) *Helper {
	k, _ := keepertest.BTCStakingKeeperWithStore(t, db, stateStore, btcStkStoreKey, btclcKeeper, btccKForBtcStaking, iKeepereeper)
	msgSrvr := keeper.NewMsgServerImpl(*k)

	fk, ctx := keepertest.FinalityKeeperWithStore(t, db, stateStore, k, iKeepereeper, btccKForFinality, finalityHooks)
	fMsgSrvr := fkeeper.NewMsgServerImpl(*fk)

	// set all parameters
	err := k.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)
	err = fk.SetParams(ctx, ftypes.DefaultParams())
	require.NoError(t, err)

	ctx = ctx.WithHeaderInfo(header.Info{Height: 1, Time: time.Now()}).WithBlockHeight(1).WithBlockTime(time.Now())

	return &Helper{
		t:   t,
		Ctx: ctx,

		BTCStakingKeeper: k,
		MsgServer:        msgSrvr,

		FinalityKeeper: fk,
		FMsgServer:     fMsgSrvr,
		FinalityHooks:  finalityHooks,

		BTCLightClientKeeper:             btclcKeeper,
		CheckpointingKeeperForBtcStaking: btccKForBtcStaking,
		CheckpointingKeeperForFinality:   btccKForFinality,
		IncentiveKeeper:                  iKeepereeper,
		Net:                              &chaincfg.SimNetParams,
	}
}

func NewHelperWithStoreIncentiveAndBank(
	t testing.TB,
	db dbm.DB,
	stateStore store.CommitMultiStore,
	btclcKeeper *types.MockBTCLightClientKeeper,
	btccKForBtcStaking *types.MockBtcCheckpointKeeper,
	btccKForFinality *ftypes.MockCheckpointingKeeper,
	iKeepereeper types.IncentiveKeeper,
	bankKeeper *costktypes.MockBankKeeper,
	btcStkStoreKey *storetypes.KVStoreKey,
) *Helper {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	k, _ := keepertest.BTCStakingKeeperWithStore(t, db, stateStore, btcStkStoreKey, btclcKeeper, btccKForBtcStaking, iKeepereeper)
	msgSrvr := keeper.NewMsgServerImpl(*k)

	// Create a mock finality incentive keeper since finality has different interface requirements
	fIncentiveKeeper := ftypes.NewMockIncentiveKeeper(ctrl)
	fIncentiveKeeper.EXPECT().AddEventBtcDelegationActivated(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	fIncentiveKeeper.EXPECT().AddEventBtcDelegationUnbonded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	fk, ctx := keepertest.FinalityKeeperWithStore(t, db, stateStore, k, fIncentiveKeeper, btccKForFinality, ftypes.NewMultiFinalityHooks())
	fMsgSrvr := fkeeper.NewMsgServerImpl(*fk)

	// set all parameters
	err := k.SetParams(ctx, types.DefaultParams())
	require.NoError(t, err)
	err = fk.SetParams(ctx, ftypes.DefaultParams())
	require.NoError(t, err)

	ctx = ctx.WithHeaderInfo(header.Info{Height: 1, Time: time.Now()}).WithBlockHeight(1).WithBlockTime(time.Now())

	return &Helper{
		t:   t,
		Ctx: ctx,

		BTCStakingKeeper: k,
		MsgServer:        msgSrvr,

		FinalityKeeper: fk,
		FMsgServer:     fMsgSrvr,

		BTCLightClientKeeper:             btclcKeeper,
		CheckpointingKeeperForBtcStaking: btccKForBtcStaking,
		CheckpointingKeeperForFinality:   btccKForFinality,
		IncentiveKeeper:                  iKeepereeper,
		Net:                              &chaincfg.SimNetParams,
	}
}

func (h *Helper) T() testing.TB {
	return h.t
}

func (h *Helper) NoError(err error) {
	require.NoError(h.t, err)
}

func (h *Helper) Equal(expected, actual interface{}) {
	require.Equal(h.t, expected, actual)
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
	// ensure that unbonding_time is larger than finalizationTimeout
	return h.GenAndApplyCustomParams(r, 100, 200, 0)
}

func (h *Helper) SetCtxHeight(height uint64) {
	h.Ctx = datagen.WithCtxHeight(h.Ctx, height)
}

func (h *Helper) GenAndApplyCustomParams(
	r *rand.Rand,
	finalizationTimeout uint32,
	unbondingTime uint32,
	allowListExpirationHeight uint64,
) ([]*btcec.PrivateKey, []*btcec.PublicKey) {
	// mock base header
	baseHeader := btclctypes.SimnetGenesisBlock()
	h.BTCLightClientKeeper.EXPECT().GetBaseBTCHeader(gomock.Any()).Return(&baseHeader).AnyTimes()

	params := btcctypes.DefaultParams()
	params.CheckpointFinalizationTimeout = finalizationTimeout

	h.CheckpointingKeeperForBtcStaking.EXPECT().GetParams(gomock.Any()).Return(params).AnyTimes()

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
		MinStakingValueSat:        10000,
		MaxStakingValueSat:        int64(4 * 10e8),
		MinStakingTimeBlocks:      400,
		MaxStakingTimeBlocks:      10000,
		SlashingPkScript:          slashingPkScript,
		MinSlashingTxFeeSat:       10,
		MinCommissionRate:         sdkmath.LegacyMustNewDecFromStr("0.01"),
		SlashingRate:              sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2),
		UnbondingTimeBlocks:       unbondingTime,
		UnbondingFeeSat:           1000,
		AllowListExpirationHeight: allowListExpirationHeight,
		BtcActivationHeight:       1,
		MaxStakerQuorum:           2,
		MaxStakerNum:              3,
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
		Commission: types.NewCommissionRates(
			*fp.Commission,
			fp.CommissionInfo.MaxRate,
			fp.CommissionInfo.MaxChangeRate,
		),
		BtcPk: fp.BtcPk,
		Pop:   fp.Pop,
	}

	_, err = h.MsgServer.CreateFinalityProvider(h.Ctx, &msgNewFp)
	h.NoError(err)
	return fpSK, fpPK, fp
}

func (h *Helper) CreateDelegation(
	r *rand.Rand,
	delSK *btcec.PrivateKey,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
	unbondingValue int64,
	unbondingTime uint16,
	usePreApproval bool,
	addToAllowList bool,
) (string, *types.MsgCreateBTCDelegation, *types.BTCDelegation, *btclctypes.BTCHeaderInfo, *types.InclusionProof, *UnbondingTxInfo, error) {
	return h.CreateDelegationWithBtcBlockHeight(
		r, delSK, fpPK, stakingValue,
		stakingTime, unbondingValue, unbondingTime,
		usePreApproval, addToAllowList, 10, 10,
	)
}

func (h *Helper) CreateDelegationWithBtcBlockHeight(
	r *rand.Rand,
	delSK *btcec.PrivateKey,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
	unbondingValue int64,
	unbondingTime uint16,
	usePreApproval bool,
	addToAllowList bool,
	stakingTransactionInclusionHeight uint32,
	lightClientTipHeight uint32,
) (string, *types.MsgCreateBTCDelegation, *types.BTCDelegation, *btclctypes.BTCHeaderInfo, *types.InclusionProof, *UnbondingTxInfo, error) {
	stakingTimeBlocks := stakingTime
	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)
	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(bsParams.CovenantPks)
	h.NoError(err)

	// if not set, use default values for unbonding value and time
	defaultUnbondingValue := stakingValue - 1000
	if unbondingValue == 0 {
		unbondingValue = defaultUnbondingValue
	}
	defaultUnbondingTime := bsParams.UnbondingTimeBlocks
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
	pop, err := datagen.NewPoPBTC(staker, delSK)
	h.NoError(err)
	// generate staking tx info
	prevBlock, _ := datagen.GenRandomBtcdBlock(r, 0, nil)
	btcHeaderWithProof := datagen.CreateBlockWithTransaction(r, &prevBlock.Header, testStakingInfo.StakingTx)
	btcHeader := btcHeaderWithProof.HeaderBytes
	btcHeaderInfo := &btclctypes.BTCHeaderInfo{Header: &btcHeader, Height: stakingTransactionInclusionHeight}
	serializedStakingTx, err := bbn.SerializeBTCTx(testStakingInfo.StakingTx)
	h.NoError(err)

	txInclusionProof := types.NewInclusionProof(&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()}, btcHeaderWithProof.SpvProof.MerkleNodes)

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
	h.BTCLightClientKeeper.EXPECT().GetHeaderByHash(gomock.Eq(h.Ctx), gomock.Eq(btcUnbondingHeader.Hash())).Return(btcUnbondingHeaderInfo, nil).AnyTimes()

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

	// mock for testing k-deep stuff
	h.BTCLightClientKeeper.EXPECT().GetHeaderByHash(gomock.Eq(h.Ctx), gomock.Eq(btcHeader.Hash())).Return(btcHeaderInfo, nil).AnyTimes()
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: lightClientTipHeight})

	_, err = h.MsgServer.CreateBTCDelegation(h.Ctx, msgCreateBTCDel)
	if err != nil {
		return "", nil, nil, nil, nil, nil, err
	}

	stakingMsgTx, err := bbn.NewBTCTxFromBytes(msgCreateBTCDel.StakingTx)
	h.NoError(err)
	btcDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingMsgTx.TxHash().String())
	h.NoError(err)

	// ensure the delegation is still pending
	status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, btcDel, bsParams.CovenantQuorum, btcTipHeight)
	require.NoError(h.t, err)
	require.Equal(h.t, status, types.BTCDelegationStatus_PENDING)

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

func (h *Helper) CreateMultisigDelegationWithBtcBlockHeight(
	r *rand.Rand,
	delSKs []*btcec.PrivateKey,
	delQuorum uint32,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
	unbondingValue int64,
	unbondingTime uint16,
	usePreApproval bool,
	addToAllowList bool,
	stakingTransactionInclusionHeight uint32,
	lightClientTipHeight uint32,
) (string, *types.MsgCreateBTCDelegation, *types.BTCDelegation, *btclctypes.BTCHeaderInfo, *types.InclusionProof, *UnbondingTxInfo, error) {
	stakingTimeBlocks := stakingTime
	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)
	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(bsParams.CovenantPks)
	h.NoError(err)

	// if not set, use default values for unbonding value and time
	defaultUnbondingValue := stakingValue - 1000
	if unbondingValue == 0 {
		unbondingValue = defaultUnbondingValue
	}
	defaultUnbondingTime := bsParams.UnbondingTimeBlocks
	if unbondingTime == 0 {
		unbondingTime = uint16(defaultUnbondingTime)
	}

	testStakingInfo := datagen.GenMultisigBTCStakingSlashingInfo(
		r,
		h.t,
		h.Net,
		delSKs,
		delQuorum,
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
	pop, err := datagen.NewPoPBTC(staker, delSKs[0])
	h.NoError(err)
	// generate staking tx info
	prevBlock, _ := datagen.GenRandomBtcdBlock(r, 0, nil)
	btcHeaderWithProof := datagen.CreateBlockWithTransaction(r, &prevBlock.Header, testStakingInfo.StakingTx)
	btcHeader := btcHeaderWithProof.HeaderBytes
	btcHeaderInfo := &btclctypes.BTCHeaderInfo{Header: &btcHeader, Height: stakingTransactionInclusionHeight}
	serializedStakingTx, err := bbn.SerializeBTCTx(testStakingInfo.StakingTx)
	h.NoError(err)

	txInclusionProof := types.NewInclusionProof(&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()}, btcHeaderWithProof.SpvProof.MerkleNodes)

	slashingSpendInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	h.NoError(err)

	// generate proper delegator sig
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		testStakingInfo.StakingTx,
		0,
		slashingSpendInfo.GetPkScriptPath(),
		delSKs[0],
	)
	h.NoError(err)

	stakerPk := delSKs[0].PubKey()
	stPk := bbn.NewBIP340PubKeyFromBTCPK(stakerPk)

	// generate extra delegator sigs
	var delegatorSI []*types.SignatureInfo
	for _, delSK := range delSKs[1:] {
		delegatorSig, err := testStakingInfo.SlashingTx.Sign(
			testStakingInfo.StakingTx,
			0,
			slashingSpendInfo.GetPkScriptPath(),
			delSK,
		)
		h.NoError(err)
		si := &types.SignatureInfo{
			Pk:  bbn.NewBIP340PubKeyFromBTCPK(delSK.PubKey()),
			Sig: delegatorSig,
		}
		delegatorSI = append(delegatorSI, si)
	}

	/*
		logics related to on-demand unbonding
	*/
	stkTxHash := testStakingInfo.StakingTx.TxHash()
	stkOutputIdx := uint32(0)

	testUnbondingInfo := datagen.GenMultisigBTCUnbondingSlashingInfo(
		r,
		h.t,
		h.Net,
		delSKs,
		delQuorum,
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

	delSlashingTxSig, err := testUnbondingInfo.GenDelSlashingTxSig(delSKs[0])
	h.NoError(err)

	// generate extra delegator unbonding tx sigs
	var delUnbondingSI []*types.SignatureInfo
	for _, delSK := range delSKs[1:] {
		delSlashingTxSig, err := testUnbondingInfo.GenDelSlashingTxSig(delSK)
		h.NoError(err)
		si := &types.SignatureInfo{
			Pk:  bbn.NewBIP340PubKeyFromBTCPK(delSK.PubKey()),
			Sig: delSlashingTxSig,
		}
		delUnbondingSI = append(delUnbondingSI, si)
	}

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
	h.BTCLightClientKeeper.EXPECT().GetHeaderByHash(gomock.Eq(h.Ctx), gomock.Eq(btcUnbondingHeader.Hash())).Return(btcUnbondingHeaderInfo, nil).AnyTimes()

	// construct extra staker info for multisig btc delegation
	stBtcPkList := make([]bbn.BIP340PubKey, len(delSKs)-1)
	for i, delSK := range delSKs[1:] {
		stBtcPkList[i] = *bbn.NewBIP340PubKeyFromBTCPK(delSK.PubKey())
	}

	multisigInfo := &types.AdditionalStakerInfo{
		StakerBtcPkList:                stBtcPkList,
		StakerQuorum:                   delQuorum,
		DelegatorSlashingSigs:          delegatorSI,
		DelegatorUnbondingSlashingSigs: delUnbondingSI,
	}

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
		MultisigInfo:                  multisigInfo,
	}

	if !usePreApproval {
		msgCreateBTCDel.StakingTxInclusionProof = txInclusionProof
	}

	if addToAllowList {
		h.BTCStakingKeeper.IndexAllowedStakingTransaction(h.Ctx, &stkTxHash)
	}

	// mock for testing k-deep stuff
	h.BTCLightClientKeeper.EXPECT().GetHeaderByHash(gomock.Eq(h.Ctx), gomock.Eq(btcHeader.Hash())).Return(btcHeaderInfo, nil).AnyTimes()
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: lightClientTipHeight})

	_, err = h.MsgServer.CreateBTCDelegation(h.Ctx, msgCreateBTCDel)
	if err != nil {
		return "", nil, nil, nil, nil, nil, err
	}

	stakingMsgTx, err := bbn.NewBTCTxFromBytes(msgCreateBTCDel.StakingTx)
	h.NoError(err)
	btcDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingMsgTx.TxHash().String())
	h.NoError(err)

	// ensure the delegation is still pending
	status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, btcDel, bsParams.CovenantQuorum, btcTipHeight)
	require.NoError(h.t, err)
	require.Equal(h.t, status, types.BTCDelegationStatus_PENDING)

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
		del.SlashingTx,
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

	covStkExpSigs := []*bbn.BIP340Signature{}
	if del.IsStakeExpansion() {
		prevTxHash, err := chainhash.NewHash(del.StkExp.PreviousStakingTxHash)
		h.NoError(err)
		prevDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, prevTxHash.String())
		h.NoError(err)
		params := h.BTCStakingKeeper.GetParams(h.Ctx)
		prevDelStakingInfo, err := prevDel.GetStakingInfo(&params, h.Net)
		h.NoError(err)
		covStkExpSigs, err = datagen.GenCovenantStakeExpSig(covenantSKs, del, prevDelStakingInfo)
		h.NoError(err)
	}

	msgs := make([]*types.MsgAddCovenantSigs, len(bsParams.CovenantPks))

	for i := 0; i < len(bsParams.CovenantPks); i++ {
		msgAddCovenantSig := &types.MsgAddCovenantSigs{
			Signer:                  del.StakerAddr,
			Pk:                      covenantSlashingTxSigs[i].CovPk,
			StakingTxHash:           stakingTxHash,
			SlashingTxSigs:          covenantSlashingTxSigs[i].AdaptorSigs,
			UnbondingTxSig:          bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
			SlashingUnbondingTxSigs: covenantUnbondingSlashingTxSigs[i].AdaptorSigs,
		}

		if del.IsStakeExpansion() {
			msgAddCovenantSig.StakeExpansionTxSig = covStkExpSigs[i]
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
	lightClientTipHeight uint32,
) {
	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

	stakingTx, err := bbn.NewBTCTxFromBytes(del.StakingTx)
	h.NoError(err)
	stakingTxHash := stakingTx.TxHash().String()

	covenantMsgs := h.GenerateCovenantSignaturesMessages(r, covenantSKs, del)
	for _, m := range covenantMsgs {
		msgCopy := m
		h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Any()).Return(&btclctypes.BTCHeaderInfo{Height: lightClientTipHeight})
		_, err := h.MsgServer.AddCovenantSigs(h.Ctx, msgCopy)
		h.NoError(err)
	}
	/*
		ensure covenant sig is added successfully
	*/
	actualDelWithCovenantSigs, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	require.Equal(h.t, len(actualDelWithCovenantSigs.CovenantSigs), len(covenantMsgs))

	hasQuorum, err := h.BTCStakingKeeper.BtcDelHasCovenantQuorums(h.Ctx, actualDelWithCovenantSigs, h.BTCStakingKeeper.GetParams(h.Ctx).CovenantQuorum)
	require.NoError(h.t, err)
	require.True(h.t, hasQuorum)

	require.NotNil(h.t, actualDelWithCovenantSigs.BtcUndelegation)
	require.NotNil(h.t, actualDelWithCovenantSigs.BtcUndelegation.CovenantSlashingSigs)
	require.NotNil(h.t, actualDelWithCovenantSigs.BtcUndelegation.CovenantUnbondingSigList)
	require.Len(h.t, actualDelWithCovenantSigs.BtcUndelegation.CovenantUnbondingSigList, len(covenantMsgs))
	require.Len(h.t, actualDelWithCovenantSigs.BtcUndelegation.CovenantSlashingSigs, len(covenantMsgs))
	require.Len(h.t, actualDelWithCovenantSigs.BtcUndelegation.CovenantSlashingSigs[0].AdaptorSigs, 1)

	// ensure the BTC delegation is verified (if using pre-approval flow) or active
	status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, actualDelWithCovenantSigs, bsParams.CovenantQuorum, btcTipHeight)
	require.NoError(h.t, err)

	if msgCreateBTCDel != nil && msgCreateBTCDel.StakingTxInclusionProof != nil {
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
	lightClientTipHeight uint32,
) {
	bsParams := h.BTCStakingKeeper.GetParams(h.Ctx)

	// Get the BTC delegation and ensure it's verified
	del, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	status, err := h.BTCStakingKeeper.BtcDelStatus(h.Ctx, del, bsParams.CovenantQuorum, btcTipHeight)
	h.NoError(err)
	require.Equal(h.t, status, types.BTCDelegationStatus_VERIFIED, "the BTC delegation shall be verified")

	// Create the MsgAddBTCDelegationInclusionProof message
	msg := &types.MsgAddBTCDelegationInclusionProof{
		StakingTxHash:           stakingTxHash,
		StakingTxInclusionProof: proof,
	}

	// mock BTC header that includes the staking tx
	h.BTCLightClientKeeper.EXPECT().GetHeaderByHash(gomock.Eq(h.Ctx), gomock.Eq(btcHeader.Header.Hash())).Return(btcHeader, nil).AnyTimes()
	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: lightClientTipHeight})

	// Call the AddBTCDelegationInclusionProof handler
	_, err = h.MsgServer.AddBTCDelegationInclusionProof(h.Ctx, msg)
	h.NoError(err)

	// Verify that the inclusion proof is added successfully and the BTC delegation
	// has been activated
	updatedDel, err := h.BTCStakingKeeper.GetBTCDelegation(h.Ctx, stakingTxHash)
	h.NoError(err)
	status, err = h.BTCStakingKeeper.BtcDelStatus(h.Ctx, updatedDel, bsParams.CovenantQuorum, btcTipHeight)
	h.NoError(err)
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

	h.CheckpointingKeeperForFinality.EXPECT().GetEpoch(gomock.Any()).Return(&epochingtypes.Epoch{EpochNumber: epoch}).Times(1)

	_, err = h.FMsgServer.CommitPubRandList(h.Ctx, msg)
	h.NoError(err)

	return randListInfo
}

func (h *Helper) AddFinalityProvider(fp *types.FinalityProvider) {
	err := h.BTCStakingKeeper.AddFinalityProvider(h.Ctx, &types.MsgCreateFinalityProvider{
		Addr:        fp.Addr,
		Description: fp.Description,
		Commission: types.NewCommissionRates(
			*fp.Commission,
			fp.CommissionInfo.MaxRate,
			fp.CommissionInfo.MaxChangeRate,
		),
		BtcPk: fp.BtcPk,
		Pop:   fp.Pop,
	})
	h.NoError(err)
}

func (h *Helper) BuildBTCInclusionProofForSpendingTx(r *rand.Rand, spendingTx *wire.MsgTx, btcHeight uint32) *types.InclusionProof {
	prevBlockForSpendingTx, _ := datagen.GenRandomBtcdBlock(r, 0, nil)
	btcHeaderWithProof := datagen.CreateBlockWithTransaction(r, &prevBlockForSpendingTx.Header, spendingTx)
	btcHeader := btcHeaderWithProof.HeaderBytes
	btcHeaderInfo := &btclctypes.BTCHeaderInfo{Header: &btcHeader, Height: btcHeight}
	spendingTxInclusionProof := types.NewInclusionProof(
		&btcctypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()},
		btcHeaderWithProof.SpvProof.MerkleNodes,
	)
	h.BTCLightClientKeeper.EXPECT().GetHeaderByHash(gomock.Eq(h.Ctx), gomock.Eq(btcHeader.Hash())).Return(btcHeaderInfo, nil).AnyTimes()
	return spendingTxInclusionProof
}

func (h *Helper) CreateBtcStakeExpansionWithBtcTipHeight(
	r *rand.Rand,
	delSK *btcec.PrivateKey,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
	prevDel *types.BTCDelegation,
	lightClientTipHeight uint32,
) (*wire.MsgTx, *wire.MsgTx, error) {
	expandMsg := h.createBtcStakeExpandMessage(
		r,
		delSK,
		fpPK,
		stakingValue,
		stakingTime,
		prevDel,
	)

	h.BTCLightClientKeeper.EXPECT().GetTipInfo(gomock.Eq(h.Ctx)).Return(&btclctypes.BTCHeaderInfo{Height: lightClientTipHeight}).MaxTimes(3)

	// Submit the BtcStakeExpand message
	_, err := h.MsgServer.BtcStakeExpand(h.Ctx, expandMsg)
	if err != nil {
		return nil, nil, err
	}

	spendingTx, err := bbn.NewBTCTxFromBytes(expandMsg.StakingTx)
	if err != nil {
		return nil, nil, err
	}

	fundingTx, err := bbn.NewBTCTxFromBytes(expandMsg.FundingTx)
	if err != nil {
		return nil, nil, err
	}

	return spendingTx, fundingTx, nil
}

// Helper function to create a BtcStakeExpand message for testing
func (h *Helper) createBtcStakeExpandMessage(
	r *rand.Rand,
	delSK *btcec.PrivateKey,
	fpPK *btcec.PublicKey,
	stakingValue int64,
	stakingTime uint16,
	prevDel *types.BTCDelegation,
) *types.MsgBtcStakeExpand {
	// Get staking parameters
	params := h.BTCStakingKeeper.GetParams(h.Ctx)

	// Convert fpPKs to BIP340PubKey format
	fpBtcPk := bbn.NewBIP340PubKeyFromBTCPK(fpPK)

	// Convert covenant keys
	var covenantPks []*btcec.PublicKey
	for _, pk := range params.CovenantPks {
		covenantPks = append(covenantPks, pk.MustToBTCPK())
	}

	// Create funding transaction
	fundingTx := datagen.GenRandomTxWithOutputValue(r, 10000000)

	// Convert previousStakingTxHash to OutPoint
	prevDelTxHash := prevDel.MustGetStakingTxHash()
	prevStakingOutPoint := wire.NewOutPoint(&prevDelTxHash, datagen.StakingOutIdx)

	// Convert fundingTxHash to OutPoint
	fundingTxHash := fundingTx.TxHash()
	fundingOutPoint := wire.NewOutPoint(&fundingTxHash, 0)
	outPoints := []*wire.OutPoint{prevStakingOutPoint, fundingOutPoint}

	// Generate staking slashing info using multiple inputs
	stakingSlashingInfo := datagen.GenBTCStakingSlashingInfoWithInputs(
		r,
		h.T(),
		h.Net,
		outPoints,
		delSK,
		[]*btcec.PublicKey{fpPK},
		covenantPks,
		params.CovenantQuorum,
		stakingTime,
		stakingValue,
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	slashingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	h.NoError(err)

	// Sign the slashing tx with delegator key
	delegatorSig, err := stakingSlashingInfo.SlashingTx.Sign(
		stakingSlashingInfo.StakingTx,
		datagen.StakingOutIdx,
		slashingPathSpendInfo.GetPkScriptPath(),
		delSK,
	)
	h.NoError(err)

	// Serialize the staking tx bytes
	serializedStakingTx, err := bbn.SerializeBTCTx(stakingSlashingInfo.StakingTx)
	h.NoError(err)

	stkTxHash := stakingSlashingInfo.StakingTx.TxHash()
	unbondingValue := uint64(stakingValue) - uint64(params.UnbondingFeeSat)

	// Generate unbonding slashing info
	unbondingSlashingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		h.T(),
		h.Net,
		delSK,
		[]*btcec.PublicKey{fpPK},
		covenantPks,
		params.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, datagen.StakingOutIdx),
		uint16(params.UnbondingTimeBlocks),
		int64(unbondingValue),
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	unbondingTxBytes, err := bbn.SerializeBTCTx(unbondingSlashingInfo.UnbondingTx)
	h.NoError(err)

	delSlashingTxSig, err := unbondingSlashingInfo.GenDelSlashingTxSig(delSK)
	h.NoError(err)

	// Create proof of possession
	stakerAddr := sdk.MustAccAddressFromBech32(prevDel.StakerAddr)
	pop, err := datagen.NewPoPBTC(stakerAddr, delSK)
	h.NoError(err)

	fundingTxBz, err := bbn.SerializeBTCTx(fundingTx)
	h.NoError(err)

	return &types.MsgBtcStakeExpand{
		StakerAddr:                    prevDel.StakerAddr,
		Pop:                           pop,
		BtcPk:                         bbn.NewBIP340PubKeyFromBTCPK(delSK.PubKey()),
		FpBtcPkList:                   []bbn.BIP340PubKey{*fpBtcPk},
		StakingTime:                   uint32(stakingTime),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		SlashingTx:                    stakingSlashingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingValue:                int64(unbondingValue),
		UnbondingTime:                 params.UnbondingTimeBlocks,
		UnbondingTx:                   unbondingTxBytes,
		UnbondingSlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delSlashingTxSig,
		PreviousStakingTxHash:         prevDelTxHash.String(),
		FundingTx:                     fundingTxBz,
	}
}
