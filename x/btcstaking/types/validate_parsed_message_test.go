package types_test

import (
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/btcstaking"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	btcckpttypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
)

// testStakingParams generates valid staking parameters with randomized
// - covenant committee members public keys
// - slashing address
func testStakingParams(
	r *rand.Rand,
	t *testing.T,
) *types.Params {
	// randomise covenant committee
	_, covenantPKs, err := datagen.GenRandomBTCKeyPairs(r, 5)
	require.NoError(t, err)
	slashingAddress, err := datagen.GenRandomBTCAddress(r, &chaincfg.MainNetParams)
	require.NoError(t, err)
	slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
	require.NoError(t, err)

	return &types.Params{
		CovenantPks:          bbn.NewBIP340PKsFromBTCPKs(covenantPKs),
		CovenantQuorum:       3,
		MinStakingValueSat:   100000,
		MaxStakingValueSat:   int64(4 * 10e8),
		MinStakingTimeBlocks: 1000,
		MaxStakingTimeBlocks: 10000,
		SlashingPkScript:     slashingPkScript,
		MinSlashingTxFeeSat:  1000,
		MinCommissionRate:    sdkmath.LegacyMustNewDecFromStr("0.01"),
		SlashingRate:         sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 41)+10), 2),
		UnbondingTimeBlocks:  200,
		UnbondingFeeSat:      1000,
	}
}

// testCheckpointParams generates valid btccheckpoint parameters
func testCheckpointParams() *btcckpttypes.Params {
	return &btcckpttypes.Params{
		BtcConfirmationDepth:          10,
		CheckpointFinalizationTimeout: 100,
	}
}

func randRange(
	r *rand.Rand,
	min, max int) int {
	return r.Intn(max-min) + min
}

type unbondingInfo struct {
	unbondingSlashingTx   *types.BTCSlashingTx
	unbondingSlashinSig   *bbn.BIP340Signature
	serializedUnbondingTx []byte
}

// generateUnbondingInfo generates valid:
// - unbonding transaction
// - unbonding slashing transaction
// - unbonding slashing transactions staker signature
func generateUnbondingInfo(
	r *rand.Rand,
	t *testing.T,
	delSK *btcec.PrivateKey,
	fpPk *btcec.PublicKey,
	stkTxHash chainhash.Hash,
	stkOutputIdx uint32,
	unbondingTime uint16,
	unbondingValue int64,
	p *types.Params,
) *unbondingInfo {
	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(p.CovenantPks)
	require.NoError(t, err)

	testUnbondingInfo := datagen.GenBTCUnbondingSlashingInfo(
		r,
		t,
		&chaincfg.MainNetParams,
		delSK,
		[]*btcec.PublicKey{fpPk},
		covPKs,
		p.CovenantQuorum,
		wire.NewOutPoint(&stkTxHash, stkOutputIdx),
		unbondingTime,
		unbondingValue,
		p.SlashingPkScript,
		p.SlashingRate,
		unbondingTime,
	)

	delSlashingTxSig, err := testUnbondingInfo.GenDelSlashingTxSig(delSK)
	require.NoError(t, err)

	serializedUnbondingTx, err := bbn.SerializeBTCTx(testUnbondingInfo.UnbondingTx)
	require.NoError(t, err)

	return &unbondingInfo{
		unbondingSlashingTx:   testUnbondingInfo.SlashingTx,
		unbondingSlashinSig:   delSlashingTxSig,
		serializedUnbondingTx: serializedUnbondingTx,
	}
}

// createMsgDelegationForParams creates a valid message to create delegation
// based on provided parameters.
// It randomly generates:
// - staker address and btc key pair
// - finality provider btc key pair
// - staking time
// - staking value
func createMsgDelegationForParams(
	r *rand.Rand,
	t *testing.T,
	p *types.Params,
) (*types.MsgCreateBTCDelegation, *btcec.PrivateKey) {
	// staker related date
	delSK, delPK, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	stPk := bbn.NewBIP340PubKeyFromBTCPK(delPK)
	staker := sdk.MustAccAddressFromBech32(datagen.GenRandomAccount().Address)
	pop, err := types.NewPoPBTC(staker, delSK)
	require.NoError(t, err)
	// finality provider related data
	_, fpPk, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)
	fpPkBBn := bbn.NewBIP340PubKeyFromBTCPK(fpPk)
	// covenants
	covPKs, err := bbn.NewBTCPKsFromBIP340PKs(p.CovenantPks)
	require.NoError(t, err)

	stakingTimeBlocks := uint16(randRange(r, int(p.MinStakingTimeBlocks), int(p.MaxStakingTimeBlocks)))
	stakingValue := int64(randRange(r, int(p.MinStakingValueSat), int(p.MaxStakingValueSat)))

	// always chose minimum unbonding time possible
	unbondingTime := p.UnbondingTimeBlocks

	testStakingInfo := datagen.GenBTCStakingSlashingInfo(
		r,
		t,
		&chaincfg.MainNetParams,
		delSK,
		[]*btcec.PublicKey{fpPk},
		covPKs,
		p.CovenantQuorum,
		stakingTimeBlocks,
		stakingValue,
		p.SlashingPkScript,
		p.SlashingRate,
		uint16(unbondingTime),
	)

	slashingSpendInfo, err := testStakingInfo.StakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	// generate proper delegator sig
	delegatorSig, err := testStakingInfo.SlashingTx.Sign(
		testStakingInfo.StakingTx,
		0,
		slashingSpendInfo.GetPkScriptPath(),
		delSK,
	)
	require.NoError(t, err)

	prevBlock, _ := datagen.GenRandomBtcdBlock(r, 0, nil)
	btcHeaderWithProof := datagen.CreateBlockWithTransaction(r, &prevBlock.Header, testStakingInfo.StakingTx)
	btcHeader := btcHeaderWithProof.HeaderBytes
	serializedStakingTx, err := bbn.SerializeBTCTx(testStakingInfo.StakingTx)
	require.NoError(t, err)

	txInclusionProof := types.NewInclusionProof(
		&btcckpttypes.TransactionKey{Index: 1, Hash: btcHeader.Hash()},
		btcHeaderWithProof.SpvProof.MerkleNodes,
	)

	stkTxHash := testStakingInfo.StakingTx.TxHash()
	stkOutputIdx := uint32(0)

	unbondingValue := stakingValue - p.UnbondingFeeSat

	unbondingInfo := generateUnbondingInfo(
		r,
		t,
		delSK,
		fpPk,
		stkTxHash,
		stkOutputIdx,
		uint16(unbondingTime),
		unbondingValue,
		p,
	)

	msgCreateBTCDel := &types.MsgCreateBTCDelegation{
		StakerAddr:                    staker.String(),
		BtcPk:                         stPk,
		FpBtcPkList:                   []bbn.BIP340PubKey{*fpPkBBn},
		Pop:                           pop,
		StakingTime:                   uint32(stakingTimeBlocks),
		StakingValue:                  stakingValue,
		StakingTx:                     serializedStakingTx,
		StakingTxInclusionProof:       txInclusionProof,
		SlashingTx:                    testStakingInfo.SlashingTx,
		DelegatorSlashingSig:          delegatorSig,
		UnbondingTx:                   unbondingInfo.serializedUnbondingTx,
		UnbondingTime:                 unbondingTime,
		UnbondingValue:                unbondingValue,
		UnbondingSlashingTx:           unbondingInfo.unbondingSlashingTx,
		DelegatorUnbondingSlashingSig: unbondingInfo.unbondingSlashinSig,
	}

	return msgCreateBTCDel, delSK
}

func TestValidateParsedMessageAgainstTheParams(t *testing.T) {
	tests := []struct {
		name          string
		fn            func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params)
		errParsing    error
		errValidation error
	}{
		{
			name: "valid create delegation message",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: nil,
		},
		{
			name: "empty finality provider list",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				msg.FpBtcPkList = []bbn.BIP340PubKey{}

				return msg, params, checkpointParams
			},
			errParsing:    types.ErrEmptyFpList,
			errValidation: nil,
		},
		{
			name: "too low unbonding time",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				msg.UnbondingTime = msg.StakingTime - 1

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidUnbondingTx,
		},
		{
			name: "Msg.BtcPk do not match pk in staking transaction",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				_, delPK, err := datagen.GenRandomBTCKeyPair(r)
				require.NoError(t, err)

				stPk := bbn.NewBIP340PubKeyFromBTCPK(delPK)

				msg.BtcPk = stPk

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidStakingTx,
		},
		{
			name: "Msg.StakingTime do not match staking time committed in staking transaction",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				msg.StakingTime++

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidStakingTx,
		},
		{
			name: "Msg.StakingValue do not match staking value committed in staking transaction",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				msg.StakingValue++

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidStakingTx.Wrap("staking tx does not contain expected staking output"),
		},
		{
			name: "Msg.StakingValue is lower than params.MinStakingValueSat",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				msg.StakingValue = params.MinStakingValueSat - 1

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidStakingTx,
		},
		{
			name: "Msg.StakingValue is higher than params.MinStakingValueSat",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				msg.StakingValue = params.MaxStakingValueSat + 1

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidStakingTx,
		},
		{
			name: "Msg.StakingTime is lower than params.MinStakingTimeBlocks",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				// modify staking output so that staking output is valid but it will have
				// invalid time
				currentStakingTx, err := bbn.NewBTCTxFromBytes(msg.StakingTx)
				require.NoError(t, err)

				invalidStakingTime := uint16(params.MinStakingTimeBlocks - 1)

				covPKs, err := bbn.NewBTCPKsFromBIP340PKs(params.CovenantPks)
				require.NoError(t, err)

				stakingOutput, err := btcstaking.BuildStakingInfo(
					msg.BtcPk.MustToBTCPK(),
					[]*btcec.PublicKey{msg.FpBtcPkList[0].MustToBTCPK()},
					covPKs,
					params.CovenantQuorum,
					invalidStakingTime,
					btcutil.Amount(msg.StakingValue),
					&chaincfg.MainNetParams,
				)
				require.NoError(t, err)

				currentStakingTx.TxOut[0] = stakingOutput.StakingOutput

				serializedNewStakingTx, err := bbn.SerializeBTCTx(currentStakingTx)
				require.NoError(t, err)

				msg.StakingTime = uint32(invalidStakingTime)
				msg.StakingTx = serializedNewStakingTx

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidStakingTx,
		},
		{
			name: "Msg.StakingTime is higher than params.MinStakingTimeBlocks",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				// modify staking output so that staking output is valid but it will have
				// invalid time
				currentStakingTx, err := bbn.NewBTCTxFromBytes(msg.StakingTx)
				require.NoError(t, err)

				invalidStakingTime := uint16(params.MaxStakingTimeBlocks + 1)

				covPKs, err := bbn.NewBTCPKsFromBIP340PKs(params.CovenantPks)
				require.NoError(t, err)

				stakingOutput, err := btcstaking.BuildStakingInfo(
					msg.BtcPk.MustToBTCPK(),
					[]*btcec.PublicKey{msg.FpBtcPkList[0].MustToBTCPK()},
					covPKs,
					params.CovenantQuorum,
					invalidStakingTime,
					btcutil.Amount(msg.StakingValue),
					&chaincfg.MainNetParams,
				)
				require.NoError(t, err)

				currentStakingTx.TxOut[0] = stakingOutput.StakingOutput

				serializedNewStakingTx, err := bbn.SerializeBTCTx(currentStakingTx)
				require.NoError(t, err)

				msg.StakingTime = uint32(invalidStakingTime)
				msg.StakingTx = serializedNewStakingTx

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidStakingTx,
		},
		{
			name: "Msg.StakingValue is lower than params.MinStakingValueSat",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				// modify staking output so that staking output is valid but it will have
				// invalid time
				currentStakingTx, err := bbn.NewBTCTxFromBytes(msg.StakingTx)
				require.NoError(t, err)

				invalidStakingValue := params.MinStakingValueSat - 1

				currentStakingTx.TxOut[0].Value = invalidStakingValue

				serializedNewStakingTx, err := bbn.SerializeBTCTx(currentStakingTx)
				require.NoError(t, err)

				msg.StakingValue = invalidStakingValue
				msg.StakingTx = serializedNewStakingTx

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidStakingTx,
		},
		{
			name: "Msg.StakingValue is higher than params.MaxStakingValueSat",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				// modify staking output so that staking output is valid but it will have
				// invalid time
				currentStakingTx, err := bbn.NewBTCTxFromBytes(msg.StakingTx)
				require.NoError(t, err)

				invalidStakingValue := params.MaxStakingValueSat + 1

				currentStakingTx.TxOut[0].Value = invalidStakingValue

				serializedNewStakingTx, err := bbn.SerializeBTCTx(currentStakingTx)
				require.NoError(t, err)

				msg.StakingValue = invalidStakingValue
				msg.StakingTx = serializedNewStakingTx

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidStakingTx,
		},
		{
			name: "Msg.SlashingTx have invalid pk script",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				currentSlashingTx, err := bbn.NewBTCTxFromBytes(*msg.SlashingTx)
				require.NoError(t, err)

				invalidSlashingPkScript := make([]byte, len(params.SlashingPkScript))
				copy(invalidSlashingPkScript, params.SlashingPkScript)
				// change one byte in the pk script
				invalidSlashingPkScript[0]++

				// slashing output must always be first output
				currentSlashingTx.TxOut[0].PkScript = invalidSlashingPkScript

				serializedNewSlashingTx, err := bbn.SerializeBTCTx(currentSlashingTx)
				require.NoError(t, err)
				msg.SlashingTx = types.NewBtcSlashingTxFromBytes(serializedNewSlashingTx)

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidStakingTx,
		},
		{
			name: "Msg.SlashingTx does not point to staking tx hash",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				currentSlashingTx, err := bbn.NewBTCTxFromBytes(*msg.SlashingTx)
				require.NoError(t, err)

				invalidHashBytes := currentSlashingTx.TxIn[0].PreviousOutPoint.Hash.CloneBytes()
				// change one byte in the hash
				invalidHashBytes[0]++

				invalidHash, err := chainhash.NewHash(invalidHashBytes)
				require.NoError(t, err)

				currentSlashingTx.TxIn[0].PreviousOutPoint.Hash = *invalidHash

				serializedNewSlashingTx, err := bbn.SerializeBTCTx(currentSlashingTx)
				require.NoError(t, err)
				msg.SlashingTx = types.NewBtcSlashingTxFromBytes(serializedNewSlashingTx)

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidStakingTx,
		},
		{
			name: "Msg.SlashingTx does not point to staking tx output index",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				currentSlashingTx, err := bbn.NewBTCTxFromBytes(*msg.SlashingTx)
				require.NoError(t, err)

				currentSlashingTx.TxIn[0].PreviousOutPoint.Index++

				serializedNewSlashingTx, err := bbn.SerializeBTCTx(currentSlashingTx)
				require.NoError(t, err)
				msg.SlashingTx = types.NewBtcSlashingTxFromBytes(serializedNewSlashingTx)

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidStakingTx,
		},
		{
			name: "Msg.DelegatorSlashingSig is invalid signature",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				sigInMessage := msg.DelegatorSlashingSig.MustMarshal()

				invalidSlashingSig := make([]byte, len(sigInMessage))
				copy(invalidSlashingSig, sigInMessage)
				// change last byte is sig
				invalidSlashingSig[63]++

				newSig, err := bbn.NewBIP340Signature(invalidSlashingSig)
				require.NoError(t, err)

				msg.DelegatorSlashingSig = newSig

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidSlashingTx,
		},
		{
			name: "Msg.UnbondingSlashingTx does not point to unbonding tx hash",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				currentSlashingTx, err := bbn.NewBTCTxFromBytes(*msg.UnbondingSlashingTx)
				require.NoError(t, err)

				invalidHashBytes := currentSlashingTx.TxIn[0].PreviousOutPoint.Hash.CloneBytes()
				// change one byte in the hash
				invalidHashBytes[0]++

				invalidHash, err := chainhash.NewHash(invalidHashBytes)
				require.NoError(t, err)

				currentSlashingTx.TxIn[0].PreviousOutPoint.Hash = *invalidHash

				serializedNewSlashingTx, err := bbn.SerializeBTCTx(currentSlashingTx)
				require.NoError(t, err)
				msg.UnbondingSlashingTx = types.NewBtcSlashingTxFromBytes(serializedNewSlashingTx)

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidUnbondingTx,
		},
		{
			name: "Msg.UnbondingSlashingTx does not point to unbonding tx output index",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				currentSlashingTx, err := bbn.NewBTCTxFromBytes(*msg.UnbondingSlashingTx)
				require.NoError(t, err)

				currentSlashingTx.TxIn[0].PreviousOutPoint.Index++

				serializedNewSlashingTx, err := bbn.SerializeBTCTx(currentSlashingTx)
				require.NoError(t, err)
				msg.UnbondingSlashingTx = types.NewBtcSlashingTxFromBytes(serializedNewSlashingTx)

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidUnbondingTx,
		},
		{
			name: "Msg.UnbondingSlashingTx have invalid pk script",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				currentUnbondingSlashingTx, err := bbn.NewBTCTxFromBytes(*msg.UnbondingSlashingTx)
				require.NoError(t, err)

				invalidSlashingPkScript := make([]byte, len(params.SlashingPkScript))
				copy(invalidSlashingPkScript, params.SlashingPkScript)
				// change one byte in the pk script
				invalidSlashingPkScript[0]++

				// slashing output must always be first output
				currentUnbondingSlashingTx.TxOut[0].PkScript = invalidSlashingPkScript

				serializedNewSlashingTx, err := bbn.SerializeBTCTx(currentUnbondingSlashingTx)
				require.NoError(t, err)
				msg.UnbondingSlashingTx = types.NewBtcSlashingTxFromBytes(serializedNewSlashingTx)

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidUnbondingTx,
		},
		{
			name: "Msg.DelegatorUnbondingSlashingSig is invalid signature",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				sigInMessage := msg.DelegatorUnbondingSlashingSig.MustMarshal()

				invalidSlashingSig := make([]byte, len(sigInMessage))
				copy(invalidSlashingSig, sigInMessage)
				// change last byte is sig
				invalidSlashingSig[63]++

				newSig, err := bbn.NewBIP340Signature(invalidSlashingSig)
				require.NoError(t, err)

				msg.DelegatorUnbondingSlashingSig = newSig

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidSlashingTx,
		},
		{
			name: "Msg.UnbondingTx does not point to staking tx hash",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, delSk := createMsgDelegationForParams(r, t, params)

				currentUnbondingTx, err := bbn.NewBTCTxFromBytes(msg.UnbondingTx)
				require.NoError(t, err)

				invalidHashBytes := currentUnbondingTx.TxIn[0].PreviousOutPoint.Hash.CloneBytes()
				// change one byte in the hash
				invalidHashBytes[0]++

				invalidHash, err := chainhash.NewHash(invalidHashBytes)
				require.NoError(t, err)

				// generate unbonding info with invalid stakig tx hash
				newUnbondingInfdo := generateUnbondingInfo(
					r,
					t,
					delSk,
					msg.FpBtcPkList[0].MustToBTCPK(),
					*invalidHash,
					0,
					uint16(msg.UnbondingTime),
					msg.UnbondingValue,
					params,
				)

				msg.UnbondingTx = newUnbondingInfdo.serializedUnbondingTx
				msg.UnbondingSlashingTx = newUnbondingInfdo.unbondingSlashingTx
				msg.DelegatorUnbondingSlashingSig = newUnbondingInfdo.unbondingSlashinSig

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidUnbondingTx,
		},
		{
			name: "Msg.UnbondingTx does not point to staking tx output index",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, delSk := createMsgDelegationForParams(r, t, params)

				currentUnbondingTx, err := bbn.NewBTCTxFromBytes(msg.UnbondingTx)
				require.NoError(t, err)

				// generate unbonding info with invalid staking idx
				newUnbondingInfdo := generateUnbondingInfo(
					r,
					t,
					delSk,
					msg.FpBtcPkList[0].MustToBTCPK(),
					currentUnbondingTx.TxIn[0].PreviousOutPoint.Hash,
					currentUnbondingTx.TxIn[0].PreviousOutPoint.Index+1,
					uint16(msg.UnbondingTime),
					msg.UnbondingValue,
					params,
				)

				msg.UnbondingTx = newUnbondingInfdo.serializedUnbondingTx
				msg.UnbondingSlashingTx = newUnbondingInfdo.unbondingSlashingTx
				msg.DelegatorUnbondingSlashingSig = newUnbondingInfdo.unbondingSlashinSig

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidUnbondingTx,
		},
		{
			name: "Msg.UnbondingTx does not have required fee",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, delSk := createMsgDelegationForParams(r, t, params)

				currentUnbondingTx, err := bbn.NewBTCTxFromBytes(msg.UnbondingTx)
				require.NoError(t, err)

				// generate unbonding info with invalid staking idx
				newUnbondingInfdo := generateUnbondingInfo(
					r,
					t,
					delSk,
					msg.FpBtcPkList[0].MustToBTCPK(),
					currentUnbondingTx.TxIn[0].PreviousOutPoint.Hash,
					currentUnbondingTx.TxIn[0].PreviousOutPoint.Index,
					uint16(msg.UnbondingTime),
					// adding 1 to unbonding value, will decrease fee by 1 sat and now it
					// won't be enough
					msg.UnbondingValue+1,
					params,
				)

				msg.UnbondingValue++
				msg.UnbondingTx = newUnbondingInfdo.serializedUnbondingTx
				msg.UnbondingSlashingTx = newUnbondingInfdo.unbondingSlashingTx
				msg.DelegatorUnbondingSlashingSig = newUnbondingInfdo.unbondingSlashinSig

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidUnbondingTx,
		},
		{
			name: "Msg.UnbondingTx has more than one output",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, _ := createMsgDelegationForParams(r, t, params)

				currentUnbondingTx, err := bbn.NewBTCTxFromBytes(msg.UnbondingTx)
				require.NoError(t, err)

				// add randomnes output
				randAddrScript, err := datagen.GenRandomPubKeyHashScript(r, &chaincfg.MainNetParams)
				require.NoError(t, err)
				currentUnbondingTx.AddTxOut(wire.NewTxOut(10000, randAddrScript))

				msg.UnbondingTx, err = bbn.SerializeBTCTx(currentUnbondingTx)
				require.NoError(t, err)

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidUnbondingTx.Wrap("unbonding tx is not a valid pre-signed transaction: tx must have exactly 1 outputs"),
		},
		{
			name: "Msg.UnbondingTx unbonding value in the msg does not match the output value in the unbonding tx",
			fn: func(r *rand.Rand, t *testing.T) (*types.MsgCreateBTCDelegation, *types.Params, *btcckpttypes.Params) {
				params := testStakingParams(r, t)
				checkpointParams := testCheckpointParams()
				msg, delSk := createMsgDelegationForParams(r, t, params)

				currentUnbondingTx, err := bbn.NewBTCTxFromBytes(msg.UnbondingTx)
				require.NoError(t, err)

				// generate unbonding info with invalid staking idx
				newUnbondingInfdo := generateUnbondingInfo(
					r,
					t,
					delSk,
					msg.FpBtcPkList[0].MustToBTCPK(),
					currentUnbondingTx.TxIn[0].PreviousOutPoint.Hash,
					currentUnbondingTx.TxIn[0].PreviousOutPoint.Index,
					uint16(msg.UnbondingTime),
					msg.UnbondingValue,
					params,
				)

				// to cause the unbonding value mismatch with the unbonding tx output value
				msg.UnbondingValue++
				msg.UnbondingTx = newUnbondingInfdo.serializedUnbondingTx
				msg.UnbondingSlashingTx = newUnbondingInfdo.unbondingSlashingTx
				msg.DelegatorUnbondingSlashingSig = newUnbondingInfdo.unbondingSlashinSig

				return msg, params, checkpointParams
			},
			errParsing:    nil,
			errValidation: types.ErrInvalidUnbondingTx.Wrap("the unbonding output value is not expected"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := rand.New(rand.NewSource(time.Now().Unix()))

			msg, params, _ := tt.fn(r, t)

			parsed, err := types.ParseCreateDelegationMessage(msg)

			if tt.errParsing != nil {
				require.Error(t, err)
				require.ErrorAs(t, err, &tt.errParsing)
				return
			}

			require.NoError(t, err)
			got, err := types.ValidateParsedMessageAgainstTheParams(
				parsed,
				params,
				&chaincfg.MainNetParams,
			)

			if tt.errValidation != nil {
				require.Error(t, err)
				require.ErrorAs(t, err, &tt.errValidation)
			} else {
				require.NoError(t, err)
				require.NotNil(t, got)
			}
		})
	}
}
