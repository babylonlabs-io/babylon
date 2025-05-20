package datagen

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	stk "github.com/babylonlabs-io/babylon/v4/btcstaking"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	"github.com/babylonlabs-io/babylon/v4/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
)

const (
	StakingOutIdx  = uint32(0)
	UnbondingTxFee = int64(1000)
)

func GenRandomFinalityProvider(r *rand.Rand) (*bstypes.FinalityProvider, error) {
	// BTC key pairs
	btcSK, _, err := GenRandomBTCKeyPair(r)
	if err != nil {
		return nil, err
	}
	return GenRandomFinalityProviderWithBTCSK(r, btcSK, "")
}

func GenRandomMsgCreateFinalityProvider(r *rand.Rand) (*bstypes.MsgCreateFinalityProvider, error) {
	// BTC key pairs
	btcSK, _, err := GenRandomBTCKeyPair(r)
	if err != nil {
		return nil, err
	}
	return GenRandomCreateFinalityProviderMsgWithBTCBabylonSKs(r, btcSK, GenRandomAccount().GetAddress())
}

func CreateNFinalityProviders(r *rand.Rand, t *testing.T, n int) []*bstypes.FinalityProvider {
	fps := make([]*bstypes.FinalityProvider, n)
	for i := 0; i < n; i++ {
		fp, err := GenRandomFinalityProvider(r)
		require.NoError(t, err)
		fps[i] = fp
	}
	return fps
}

func GenRandomFinalityProviderWithBTCSK(r *rand.Rand, btcSK *btcec.PrivateKey, consumerID string) (*bstypes.FinalityProvider, error) {
	return GenCustomFinalityProvider(r, btcSK, GenRandomAccount().GetAddress(), consumerID)
}

func GenRandomCommission(r *rand.Rand) sdkmath.LegacyDec {
	return sdkmath.LegacyNewDecWithPrec(int64(RandomInt(r, 49)+1), 2) // [1/100, 50/100]
}

func GenRandomDescription(r *rand.Rand) *stakingtypes.Description {
	return &stakingtypes.Description{Moniker: GenRandomHexStr(r, 10)}
}

func GenCustomFinalityProvider(r *rand.Rand, btcSK *btcec.PrivateKey, fpAddr sdk.AccAddress, consumerID string) (*bstypes.FinalityProvider, error) {
	// commission
	commission := GenRandomCommission(r)
	// description
	description := GenRandomDescription(r)
	// key pairs
	btcPK := btcSK.PubKey()
	bip340PK := bbn.NewBIP340PubKeyFromBTCPK(btcPK)
	// pop
	pop, err := NewPoPBTC(fpAddr, btcSK)
	if err != nil {
		return nil, err
	}
	return &bstypes.FinalityProvider{
		Description: description,
		Commission:  &commission,
		BtcPk:       bip340PK,
		Addr:        fpAddr.String(),
		Pop:         pop,
		ConsumerId:  consumerID,
		CommissionInfo: bstypes.NewCommissionInfoWithTime(
			sdkmath.LegacyOneDec(),
			sdkmath.LegacyOneDec(),
			time.Unix(0, 0).UTC(),
		),
	}, nil
}

func GenRandomCreateFinalityProviderMsgWithBTCBabylonSKs(
	r *rand.Rand,
	btcSK *btcec.PrivateKey,
	fpAddr sdk.AccAddress,
) (*bstypes.MsgCreateFinalityProvider, error) {
	fp, err := GenCustomFinalityProvider(r, btcSK, fpAddr, "")
	if err != nil {
		return nil, err
	}
	return &bstypes.MsgCreateFinalityProvider{
		Addr:        fp.Addr,
		Description: fp.Description,
		Commission: types.NewCommissionRates(
			*fp.Commission,
			fp.CommissionInfo.MaxRate,
			fp.CommissionInfo.MaxChangeRate,
		),
		BtcPk: fp.BtcPk,
		Pop:   fp.Pop,
	}, nil
}

// TODO: accommodate presign unbonding flow
func GenRandomBTCDelegation(
	r *rand.Rand,
	t *testing.T,
	btcNet *chaincfg.Params,
	fpBTCPKs []bbn.BIP340PubKey,
	delSK *btcec.PrivateKey,
	covenantSKs []*btcec.PrivateKey,
	covenantPks []*btcec.PublicKey,
	covenantQuorum uint32,
	slashingPkScript []byte,
	stakingTime, startHeight, endHeight uint32,
	totalSat uint64,
	slashingRate sdkmath.LegacyDec,
	slashingChangeLockTime uint16,
) (*bstypes.BTCDelegation, error) {
	delPK := delSK.PubKey()
	delBTCPK := bbn.NewBIP340PubKeyFromBTCPK(delPK)

	// list of finality provider PKs
	fpPKs, err := bbn.NewBTCPKsFromBIP340PKs(fpBTCPKs)
	if err != nil {
		return nil, err
	}

	stakerAddress := GenRandomSecp256k1Address()

	// staking/slashing tx
	stakingSlashingInfo := GenBTCStakingSlashingInfo(
		r,
		t,
		btcNet,
		delSK,
		fpPKs,
		covenantPks,
		covenantQuorum,
		uint16(stakingTime),
		int64(totalSat),
		slashingPkScript,
		slashingRate,
		slashingChangeLockTime,
	)

	slashingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// delegator pre-signs slashing tx
	delegatorSig, err := stakingSlashingInfo.SlashingTx.Sign(
		stakingSlashingInfo.StakingTx,
		StakingOutIdx,
		slashingPathSpendInfo.GetPkScriptPath(),
		delSK,
	)
	require.NoError(t, err)

	// covenant pre-signs slashing tx
	covenantSigs, err := GenCovenantAdaptorSigs(
		covenantSKs,
		fpPKs,
		stakingSlashingInfo.StakingTx,
		slashingPathSpendInfo.GetPkScriptPath(),
		stakingSlashingInfo.SlashingTx,
	)
	require.NoError(t, err)

	serializedStakingTx, err := bbn.SerializeBTCTx(stakingSlashingInfo.StakingTx)
	require.NoError(t, err)
	w := uint16(100) // TODO: parameterise w

	pop, err := NewPoPBTC(stakerAddress, delSK)
	require.NoError(t, err)

	del := &bstypes.BTCDelegation{
		StakerAddr:       sdk.MustBech32ifyAddressBytes(appparams.Bech32PrefixAccAddr, stakerAddress), // Staker address is always Babylon's
		BtcPk:            delBTCPK,
		Pop:              pop,
		FpBtcPkList:      fpBTCPKs,
		StakingTime:      stakingTime,
		StartHeight:      startHeight,
		EndHeight:        endHeight,
		TotalSat:         totalSat,
		StakingOutputIdx: StakingOutIdx,
		DelegatorSig:     delegatorSig,
		CovenantSigs:     covenantSigs,
		UnbondingTime:    uint32(w + 1),
		StakingTx:        serializedStakingTx,
		SlashingTx:       stakingSlashingInfo.SlashingTx,
	}

	/*
		construct BTC undelegation
	*/

	// construct unbonding info
	stkTxHash := stakingSlashingInfo.StakingTx.TxHash()
	unbondingValue := totalSat - uint64(UnbondingTxFee)

	unbondingSlashingInfo := GenBTCUnbondingSlashingInfo(
		r,
		t,
		btcNet,
		delSK,
		fpPKs,
		covenantPks,
		covenantQuorum,
		wire.NewOutPoint(&stkTxHash, StakingOutIdx),
		w+1,
		int64(unbondingValue),
		slashingPkScript,
		slashingRate,
		slashingChangeLockTime,
	)

	unbondingTxBytes, err := bbn.SerializeBTCTx(unbondingSlashingInfo.UnbondingTx)
	require.NoError(t, err)
	delSlashingTxSig, err := unbondingSlashingInfo.GenDelSlashingTxSig(delSK)
	require.NoError(t, err)
	del.BtcUndelegation = &bstypes.BTCUndelegation{
		UnbondingTx:          unbondingTxBytes,
		SlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorSlashingSig: delSlashingTxSig,
	}

	/*
		covenant signs BTC undelegation
	*/

	unbondingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)

	covUnbondingSlashingSigs, covUnbondingSigs, err := unbondingSlashingInfo.GenCovenantSigs(
		covenantSKs,
		fpPKs,
		stakingSlashingInfo.StakingTx,
		unbondingPathSpendInfo.GetPkScriptPath(),
	)
	require.NoError(t, err)

	del.BtcUndelegation.CovenantSlashingSigs = covUnbondingSlashingSigs
	del.BtcUndelegation.CovenantUnbondingSigList = covUnbondingSigs

	return del, nil
}

type CreateDelegationInfo struct {
	MsgCreateBTCDelegation *bstypes.MsgCreateBTCDelegation
	MsgAddCovenantSigs     []*bstypes.MsgAddCovenantSigs
	StakingTxHash          string
	StakingTx              *wire.MsgTx
	UnbondingTx            *wire.MsgTx
}

func DelegationInfosToBTCTx(
	delInfos []*CreateDelegationInfo,
) []*wire.MsgTx {
	txs := []*wire.MsgTx{}
	for _, delInfo := range delInfos {
		txs = append(txs, delInfo.StakingTx)
	}
	return txs
}

// GenRandomMsgCreateBtcDelegation generates a random MsgCreateBTCDelegation message
// valid for the given parameters.
func GenRandomMsgCreateBtcDelegationAndMsgAddCovenantSignatures(
	r *rand.Rand,
	t *testing.T,
	btcNet *chaincfg.Params,
	stakerAddr sdk.AccAddress,
	fpBTCPKs []bbn.BIP340PubKey,
	delSK *btcec.PrivateKey,
	covenantSKs []*btcec.PrivateKey,
	params *bstypes.Params,
) *CreateDelegationInfo {
	require.Positive(t, params.CovenantQuorum)
	require.Positive(t, len(fpBTCPKs))
	require.Positive(t, len(covenantSKs))
	require.Equal(t, len(params.CovenantPks), len(covenantSKs))

	delPK := delSK.PubKey()

	delBTCPK := bbn.NewBIP340PubKeyFromBTCPK(delPK)
	// list of finality provider PKs
	fpPKs, err := bbn.NewBTCPKsFromBIP340PKs(fpBTCPKs)
	require.NoError(t, err)
	var covenantPks []*btcec.PublicKey
	for _, sk := range covenantSKs {
		covenantPks = append(covenantPks, sk.PubKey())
	}

	stakingTime := RandomInRange(
		r,
		int(params.MinStakingTimeBlocks),
		int(params.MaxStakingTimeBlocks),
	)

	totalSat := RandomInRange(
		r,
		// add 10000 just in case
		int(params.MinStakingValueSat)+10000,
		int(params.MaxStakingValueSat),
	)

	// staking/slashing tx
	stakingSlashingInfo := GenBTCStakingSlashingInfo(
		r,
		t,
		btcNet,
		delSK,
		fpPKs,
		covenantPks,
		params.CovenantQuorum,
		uint16(stakingTime),
		int64(totalSat),
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	slashingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// delegator pre-signs slashing tx
	delegatorSig, err := stakingSlashingInfo.SlashingTx.Sign(
		stakingSlashingInfo.StakingTx,
		StakingOutIdx,
		slashingPathSpendInfo.GetPkScriptPath(),
		delSK,
	)
	require.NoError(t, err)

	serializedStakingTx, err := bbn.SerializeBTCTx(stakingSlashingInfo.StakingTx)
	require.NoError(t, err)

	stkTxHash := stakingSlashingInfo.StakingTx.TxHash()
	unbondingValue := uint64(totalSat) - uint64(params.UnbondingFeeSat)

	unbondingSlashingInfo := GenBTCUnbondingSlashingInfo(
		r,
		t,
		btcNet,
		delSK,
		fpPKs,
		covenantPks,
		params.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, StakingOutIdx),
		uint16(params.UnbondingTimeBlocks),
		int64(unbondingValue),
		params.SlashingPkScript,
		params.SlashingRate,
		uint16(params.UnbondingTimeBlocks),
	)

	unbondingTxBytes, err := bbn.SerializeBTCTx(unbondingSlashingInfo.UnbondingTx)
	require.NoError(t, err)
	delSlashingTxSig, err := unbondingSlashingInfo.GenDelSlashingTxSig(delSK)
	require.NoError(t, err)

	pop, err := NewPoPBTC(sdk.MustAccAddressFromBech32(stakerAddr.String()), delSK)
	require.NoError(t, err)

	msg := &bstypes.MsgCreateBTCDelegation{
		StakerAddr:   stakerAddr.String(),
		Pop:          pop,
		BtcPk:        delBTCPK,
		FpBtcPkList:  fpBTCPKs,
		StakingTime:  uint32(stakingTime),
		StakingValue: int64(totalSat),
		StakingTx:    serializedStakingTx,
		// By default it is nil it is up to the caller to set it
		StakingTxInclusionProof:       nil,
		SlashingTx:                    stakingSlashingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingValue:                int64(unbondingValue),
		UnbondingTime:                 params.UnbondingTimeBlocks,
		UnbondingTx:                   unbondingTxBytes,
		UnbondingSlashingTx:           unbondingSlashingInfo.SlashingTx,
		DelegatorUnbondingSlashingSig: delSlashingTxSig,
	}

	// covenant pre-signs slashing tx for staking tx
	covenantSigs, err := GenCovenantAdaptorSigs(
		covenantSKs,
		fpPKs,
		stakingSlashingInfo.StakingTx,
		slashingPathSpendInfo.GetPkScriptPath(),
		stakingSlashingInfo.SlashingTx,
	)
	require.NoError(t, err)

	// covenant pre-signs slashing tx and unbonding tx
	unbondingPathSpendInfo, err := stakingSlashingInfo.StakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)
	covUnbondingSlashingSigs, covUnbondingSigs, err := unbondingSlashingInfo.GenCovenantSigs(
		covenantSKs,
		fpPKs,
		stakingSlashingInfo.StakingTx,
		unbondingPathSpendInfo.GetPkScriptPath(),
	)
	require.NoError(t, err)

	msgs := make([]*bstypes.MsgAddCovenantSigs, len(covenantPks))

	for i := 0; i < len(covenantPks); i++ {
		msgAddCovenantSig := &bstypes.MsgAddCovenantSigs{
			Signer:                  stakerAddr.String(),
			Pk:                      covenantSigs[i].CovPk,
			StakingTxHash:           stkTxHash.String(),
			SlashingTxSigs:          covenantSigs[i].AdaptorSigs,
			UnbondingTxSig:          covUnbondingSigs[i].Sig,
			SlashingUnbondingTxSigs: covUnbondingSlashingSigs[i].AdaptorSigs,
		}
		msgs[i] = msgAddCovenantSig
	}

	return &CreateDelegationInfo{
		MsgCreateBTCDelegation: msg,
		MsgAddCovenantSigs:     msgs,
		StakingTxHash:          stkTxHash.String(),
		StakingTx:              stakingSlashingInfo.StakingTx,
		UnbondingTx:            unbondingSlashingInfo.UnbondingTx,
	}
}

type TestStakingSlashingInfo struct {
	StakingTx   *wire.MsgTx
	SlashingTx  *bstypes.BTCSlashingTx
	StakingInfo *btcstaking.StakingInfo
}

type TestUnbondingSlashingInfo struct {
	UnbondingTx   *wire.MsgTx
	SlashingTx    *bstypes.BTCSlashingTx
	UnbondingInfo *btcstaking.UnbondingInfo
}

func GenBTCStakingSlashingInfoWithOutPoint(
	r *rand.Rand,
	t testing.TB,
	btcNet *chaincfg.Params,
	outPoint *wire.OutPoint,
	stakerSK *btcec.PrivateKey,
	fpPKs []*btcec.PublicKey,
	covenantPKs []*btcec.PublicKey,
	covenantQuorum uint32,
	stakingTimeBlocks uint16,
	stakingValue int64,
	slashingPkScript []byte,
	slashingRate sdkmath.LegacyDec,
	slashingChangeLockTime uint16,
) *TestStakingSlashingInfo {
	stakingInfo, err := btcstaking.BuildStakingInfo(
		stakerSK.PubKey(),
		fpPKs,
		covenantPKs,
		covenantQuorum,
		stakingTimeBlocks,
		btcutil.Amount(stakingValue),
		btcNet,
	)

	require.NoError(t, err)
	tx := wire.NewMsgTx(2)
	// add the given tx input
	txIn := wire.NewTxIn(outPoint, nil, nil)
	tx.AddTxIn(txIn)
	tx.AddTxOut(stakingInfo.StakingOutput)

	// 2 outputs for changes and staking output
	changeAddrScript, err := GenRandomPubKeyHashScript(r, btcNet)
	require.NoError(t, err)
	require.False(t, txscript.GetScriptClass(changeAddrScript) == txscript.NonStandardTy)

	tx.AddTxOut(wire.NewTxOut(10000, changeAddrScript)) // output for change

	slashingMsgTx, err := btcstaking.BuildSlashingTxFromStakingTxStrict(
		tx,
		StakingOutIdx,
		slashingPkScript,
		stakerSK.PubKey(),
		slashingChangeLockTime,
		2000,
		slashingRate,
		btcNet)
	require.NoError(t, err)
	slashingTx, err := bstypes.NewBTCSlashingTxFromMsgTx(slashingMsgTx)
	require.NoError(t, err)

	return &TestStakingSlashingInfo{
		StakingTx:   tx,
		SlashingTx:  slashingTx,
		StakingInfo: stakingInfo,
	}
}

func GenBTCStakingSlashingInfo(
	r *rand.Rand,
	t testing.TB,
	btcNet *chaincfg.Params,
	stakerSK *btcec.PrivateKey,
	fpPKs []*btcec.PublicKey,
	covenantPKs []*btcec.PublicKey,
	covenantQuorum uint32,
	stakingTimeBlocks uint16,
	stakingValue int64,
	slashingPkScript []byte,
	slashingRate sdkmath.LegacyDec,
	slashingChangeLockTime uint16,
) *TestStakingSlashingInfo {
	// an arbitrary input
	spend := makeSpendableOutWithRandOutPoint(r, btcutil.Amount(stakingValue+UnbondingTxFee))
	outPoint := &spend.prevOut
	return GenBTCStakingSlashingInfoWithOutPoint(
		r,
		t,
		btcNet,
		outPoint,
		stakerSK,
		fpPKs,
		covenantPKs,
		covenantQuorum,
		stakingTimeBlocks,
		stakingValue,
		slashingPkScript,
		slashingRate,
		slashingChangeLockTime,
	)
}

func GenBTCUnbondingSlashingInfo(
	r *rand.Rand,
	t testing.TB,
	btcNet *chaincfg.Params,
	stakerSK *btcec.PrivateKey,
	fpPKs []*btcec.PublicKey,
	covenantPKs []*btcec.PublicKey,
	covenantQuorum uint32,
	stakingTransactionOutpoint *wire.OutPoint,
	stakingTimeBlocks uint16,
	stakingValue int64,
	slashingPkScript []byte,
	slashingRate sdkmath.LegacyDec,
	slashingChangeLockTime uint16,
) *TestUnbondingSlashingInfo {
	unbondingInfo, err := btcstaking.BuildUnbondingInfo(
		stakerSK.PubKey(),
		fpPKs,
		covenantPKs,
		covenantQuorum,
		slashingChangeLockTime,
		btcutil.Amount(stakingValue),
		btcNet,
	)

	require.NoError(t, err)
	tx := wire.NewMsgTx(2)
	// add the given tx input
	txIn := wire.NewTxIn(stakingTransactionOutpoint, nil, nil)
	tx.AddTxIn(txIn)
	tx.AddTxOut(unbondingInfo.UnbondingOutput)

	slashingMsgTx, err := btcstaking.BuildSlashingTxFromStakingTxStrict(
		tx,
		StakingOutIdx,
		slashingPkScript,
		stakerSK.PubKey(),
		slashingChangeLockTime,
		2000,
		slashingRate,
		btcNet)
	require.NoError(t, err)
	slashingTx, err := bstypes.NewBTCSlashingTxFromMsgTx(slashingMsgTx)
	require.NoError(t, err)

	return &TestUnbondingSlashingInfo{
		UnbondingTx:   tx,
		SlashingTx:    slashingTx,
		UnbondingInfo: unbondingInfo,
	}
}

func (info *TestUnbondingSlashingInfo) GenDelSlashingTxSig(sk *btcec.PrivateKey) (*bbn.BIP340Signature, error) {
	unbondingTxMsg := info.UnbondingTx
	unbondingTxSlashingPathInfo, err := info.UnbondingInfo.SlashingPathSpendInfo()
	if err != nil {
		return nil, err
	}
	slashingTxSig, err := info.SlashingTx.Sign(
		unbondingTxMsg,
		StakingOutIdx,
		unbondingTxSlashingPathInfo.GetPkScriptPath(),
		sk,
	)
	if err != nil {
		return nil, err
	}
	return slashingTxSig, nil
}

func (info *TestUnbondingSlashingInfo) GenCovenantSigs(
	covSKs []*btcec.PrivateKey,
	fpPKs []*btcec.PublicKey,
	stakingTx *wire.MsgTx,
	unbondingPkScriptPath []byte,
) ([]*bstypes.CovenantAdaptorSignatures, []*bstypes.SignatureInfo, error) {
	unbondingSlashingPathInfo, err := info.UnbondingInfo.SlashingPathSpendInfo()
	if err != nil {
		return nil, nil, err
	}

	covUnbondingSlashingSigs, err := GenCovenantAdaptorSigs(
		covSKs,
		fpPKs,
		info.UnbondingTx,
		unbondingSlashingPathInfo.GetPkScriptPath(),
		info.SlashingTx,
	)
	if err != nil {
		return nil, nil, err
	}
	covUnbondingSigs, err := GenCovenantUnbondingSigs(
		covSKs,
		stakingTx,
		StakingOutIdx,
		unbondingPkScriptPath,
		info.UnbondingTx,
	)
	if err != nil {
		return nil, nil, err
	}
	covUnbondingSigList := []*bstypes.SignatureInfo{}
	for i := range covUnbondingSigs {
		covUnbondingSigList = append(covUnbondingSigList, &bstypes.SignatureInfo{
			Pk:  bbn.NewBIP340PubKeyFromBTCPK(covSKs[i].PubKey()),
			Sig: bbn.NewBIP340SignatureFromBTCSig(covUnbondingSigs[i]),
		})
	}
	return covUnbondingSlashingSigs, covUnbondingSigList, nil
}

type SignatureInfo struct {
	SignerPubKey *btcec.PublicKey
	Signature    *schnorr.Signature
}

func NewSignatureInfo(
	signerPubKey *btcec.PublicKey,
	signature *schnorr.Signature,
) *SignatureInfo {
	return &SignatureInfo{
		SignerPubKey: signerPubKey,
		Signature:    signature,
	}
}

// Helper function to sort all signatures in reverse lexicographical order of signing public keys
// this way signatures are ready to be used in multisig witness with corresponding public keys
func sortSignatureInfo(infos []*SignatureInfo) []*SignatureInfo {
	sortedInfos := make([]*SignatureInfo, len(infos))
	copy(sortedInfos, infos)
	sort.SliceStable(sortedInfos, func(i, j int) bool {
		keyIBytes := schnorr.SerializePubKey(sortedInfos[i].SignerPubKey)
		keyJBytes := schnorr.SerializePubKey(sortedInfos[j].SignerPubKey)
		return bytes.Compare(keyIBytes, keyJBytes) == 1
	})

	return sortedInfos
}

// generate list of signatures in valid order
func GenerateSignatures(
	t *testing.T,
	keys []*btcec.PrivateKey,
	tx *wire.MsgTx,
	stakingOutput *wire.TxOut,
	leaf txscript.TapLeaf,
) []*schnorr.Signature {
	var si []*SignatureInfo

	for _, key := range keys {
		pubKey := key.PubKey()
		sig, err := btcstaking.SignTxWithOneScriptSpendInputFromTapLeaf(
			tx,
			stakingOutput,
			key,
			leaf,
		)
		require.NoError(t, err)
		info := NewSignatureInfo(
			pubKey,
			sig,
		)
		si = append(si, info)
	}

	// sort signatures by public key
	sortedSigInfo := sortSignatureInfo(si)

	var sigs []*schnorr.Signature = make([]*schnorr.Signature, len(sortedSigInfo))

	for i, sigInfo := range sortedSigInfo {
		sig := sigInfo
		sigs[i] = sig.Signature
	}

	return sigs
}

func AddWitnessToUnbondingTx(
	t *testing.T,
	stakingOutput *wire.TxOut,
	stakerSk *btcec.PrivateKey,
	covenantSks []*btcec.PrivateKey,
	covenantQuorum uint32,
	finalityProviderPKs []*btcec.PublicKey,
	stakingTime uint16,
	stakingValue int64,
	unbondingTx *wire.MsgTx,
	net *chaincfg.Params,
) ([]byte, *wire.MsgTx) {
	var covenatnPks []*btcec.PublicKey
	for _, sk := range covenantSks {
		covenatnPks = append(covenatnPks, sk.PubKey())
	}

	stakingInfo, err := stk.BuildStakingInfo(
		stakerSk.PubKey(),
		finalityProviderPKs,
		covenatnPks,
		covenantQuorum,
		stakingTime,
		btcutil.Amount(stakingValue),
		net,
	)
	require.NoError(t, err)

	// sanity check that what we re-build is the same as what we have in the BTC delegation
	require.Equal(t, stakingOutput, stakingInfo.StakingOutput)

	unbondingSpendInfo, err := stakingInfo.UnbondingPathSpendInfo()
	require.NoError(t, err)

	unbondingScirpt := unbondingSpendInfo.RevealedLeaf.Script
	require.NotNil(t, unbondingScirpt)

	covenantSigs := GenerateSignatures(
		t,
		covenantSks,
		unbondingTx,
		stakingOutput,
		unbondingSpendInfo.RevealedLeaf,
	)
	require.NoError(t, err)

	stakerSig, err := stk.SignTxWithOneScriptSpendInputFromTapLeaf(
		unbondingTx,
		stakingOutput,
		stakerSk,
		unbondingSpendInfo.RevealedLeaf,
	)
	require.NoError(t, err)

	ubWitness, err := unbondingSpendInfo.CreateUnbondingPathWitness(covenantSigs, stakerSig)
	require.NoError(t, err)

	unbondingTx.TxIn[0].Witness = ubWitness

	serializedUnbondingTxWithWitness, err := bbn.SerializeBTCTx(unbondingTx)
	require.NoError(t, err)

	return serializedUnbondingTxWithWitness, unbondingTx
}
