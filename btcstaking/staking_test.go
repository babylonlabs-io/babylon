package btcstaking_test

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/btcstaking"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/mempool"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

// StakingScriptData is a struct that holds data parsed from staking script
type StakingScriptData struct {
	StakerKey           *btcec.PublicKey
	FinalityProviderKey *btcec.PublicKey
	CovenantKey         *btcec.PublicKey
	StakingTime         uint16
}

func NewStakingScriptData(
	stakerKey,
	fpKey,
	covenantKey *btcec.PublicKey,
	stakingTime uint16) (*StakingScriptData, error) {
	if stakerKey == nil || fpKey == nil || covenantKey == nil {
		return nil, fmt.Errorf("staker, finality provider and covenant keys cannot be nil")
	}

	return &StakingScriptData{
		StakerKey:           stakerKey,
		FinalityProviderKey: fpKey,
		CovenantKey:         covenantKey,
		StakingTime:         stakingTime,
	}, nil
}

func genValidStakingScriptData(_ *testing.T, r *rand.Rand) *StakingScriptData {
	stakerPrivKeyBytes := datagen.GenRandomByteArray(r, 32)
	fpPrivKeyBytes := datagen.GenRandomByteArray(r, 32)
	covenantPrivKeyBytes := datagen.GenRandomByteArray(r, 32)
	stakingTime := uint16(r.Intn(math.MaxUint16))

	_, stakerPublicKey := btcec.PrivKeyFromBytes(stakerPrivKeyBytes)
	_, fpPublicKey := btcec.PrivKeyFromBytes(fpPrivKeyBytes)
	_, covenantPublicKey := btcec.PrivKeyFromBytes(covenantPrivKeyBytes)

	sd, _ := NewStakingScriptData(stakerPublicKey, fpPublicKey, covenantPublicKey, stakingTime)

	return sd
}

func FuzzGeneratingValidStakingSlashingTx(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		// we do not care for inputs in staking tx
		stakingTx := wire.NewMsgTx(2)
		bogusInputHashBytes := [32]byte{}
		bogusInputHash, _ := chainhash.NewHash(bogusInputHashBytes[:])
		stakingTx.AddTxIn(
			wire.NewTxIn(wire.NewOutPoint(bogusInputHash, 0), nil, nil),
		)

		stakingOutputIdx := r.Intn(5)
		// always more outputs than stakingOutputIdx
		stakingTxNumOutputs := r.Intn(5) + 10
		slashingLockTime := uint16(r.Intn(1000) + 1)
		sd := genValidStakingScriptData(t, r)

		minStakingValue := 5000
		minFee := 2000
		// generate a random slashing rate with random precision,
		// this will include both valid and invalid ranges, so we can test both cases
		randomPrecision := r.Int63n(6)                                                                   // [0,3]
		slashingRate := sdkmath.LegacyNewDecWithPrec(int64(datagen.RandomInt(r, 1001)), randomPrecision) // [0,1000] / 10^{randomPrecision}

		for i := 0; i < stakingTxNumOutputs; i++ {
			if i == stakingOutputIdx {
				info, err := btcstaking.BuildStakingInfo(
					sd.StakerKey,
					[]*btcec.PublicKey{sd.FinalityProviderKey},
					[]*btcec.PublicKey{sd.CovenantKey},
					1,
					sd.StakingTime,
					btcutil.Amount(r.Intn(5000)+minStakingValue),
					&chaincfg.MainNetParams,
				)

				require.NoError(t, err)
				stakingTx.AddTxOut(info.StakingOutput)
			} else {
				stakingTx.AddTxOut(
					&wire.TxOut{
						PkScript: datagen.GenRandomByteArray(r, 32),
						Value:    int64(r.Intn(5000) + 1),
					},
				)
			}
		}

		// Always check case with min fee
		testSlashingTx(r, t, stakingTx, stakingOutputIdx, slashingRate, int64(minFee), sd.StakerKey, slashingLockTime)

		// Check case with some random fee
		fee := int64(r.Intn(1000) + minFee)
		testSlashingTx(r, t, stakingTx, stakingOutputIdx, slashingRate, fee, sd.StakerKey, slashingLockTime)
	})
}

func genRandomBTCAddress(r *rand.Rand) (*btcutil.AddressPubKeyHash, error) {
	return btcutil.NewAddressPubKeyHash(datagen.GenRandomByteArray(r, 20), &chaincfg.MainNetParams)
}

func taprootOutputWithValue(t *testing.T, r *rand.Rand, value btcutil.Amount) *wire.TxOut {
	bytes := datagen.GenRandomByteArray(r, 32)
	addrr, err := btcutil.NewAddressTaproot(bytes, &chaincfg.MainNetParams)
	require.NoError(t, err)
	return outputFromAddressAndValue(t, addrr, value)
}

func outputFromAddressAndValue(t *testing.T, addr btcutil.Address, value btcutil.Amount) *wire.TxOut {
	pkScript, err := txscript.PayToAddrScript(addr)
	require.NoError(t, err)
	return wire.NewTxOut(int64(value), pkScript)
}

func testSlashingTx(
	r *rand.Rand,
	t *testing.T,
	stakingTx *wire.MsgTx,
	stakingOutputIdx int,
	slashingRate sdkmath.LegacyDec,
	fee int64,
	stakerPk *btcec.PublicKey,
	slashingChangeLockTime uint16,
) {
	// Generate random slashing and change addresses
	slashingAddress, err := genRandomBTCAddress(r)
	require.NoError(t, err)

	slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
	require.NoError(t, err)

	// Construct slashing transaction using the provided parameters
	slashingTx, err := btcstaking.BuildSlashingTxFromStakingTxStrict(
		stakingTx,
		uint32(stakingOutputIdx),
		slashingPkScript,
		stakerPk,
		slashingChangeLockTime,
		fee,
		slashingRate,
		&chaincfg.MainNetParams,
	)

	if btcstaking.IsSlashingRateValid(slashingRate) {
		// If the slashing rate is valid i.e., in the range (0,1) with at most 2 decimal places,
		// it is still possible that the slashing transaction is invalid. The following checks will confirm that
		// slashing tx is not constructed if
		// - the change output has insufficient funds.
		// - the change output is less than the dust threshold.
		// - The slashing output is less than the dust threshold.

		slashingRateFloat64, err2 := slashingRate.Float64()
		require.NoError(t, err2)

		stakingAmount := btcutil.Amount(stakingTx.TxOut[stakingOutputIdx].Value)
		slashingAmount := stakingAmount.MulF64(slashingRateFloat64)
		changeAmount := stakingAmount - slashingAmount - btcutil.Amount(fee)

		// check if the created outputs are not dust
		slashingOutput := outputFromAddressAndValue(t, slashingAddress, slashingAmount)
		changeOutput := taprootOutputWithValue(t, r, changeAmount)

		switch {
		case changeAmount <= 0:
			require.Error(t, err)
			require.ErrorIs(t, err, btcstaking.ErrInsufficientChangeAmount)
		case mempool.IsDust(slashingOutput, mempool.DefaultMinRelayTxFee) || mempool.IsDust(changeOutput, mempool.DefaultMinRelayTxFee):
			require.Error(t, err)
			require.ErrorIs(t, err, btcstaking.ErrDustOutputFound)
		default:
			require.NoError(t, err)
			err = btcstaking.CheckSlashingTxMatchFundingTx(
				slashingTx,
				stakingTx,
				uint32(stakingOutputIdx),
				fee,
				slashingRate,
				slashingPkScript,
				stakerPk,
				slashingChangeLockTime,
				&chaincfg.MainNetParams,
			)
			require.NoError(t, err)
		}
	} else {
		require.Error(t, err)
		require.ErrorIs(t, err, btcstaking.ErrInvalidSlashingRate)
	}
}

func FuzzGeneratingSignatureValidation(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		pk, err := btcec.NewPrivateKey()
		require.NoError(t, err)
		inputHash, err := chainhash.NewHash(datagen.GenRandomByteArray(r, 32))
		require.NoError(t, err)

		tx := wire.NewMsgTx(2)
		foundingOutput := wire.NewTxOut(int64(r.Intn(1000)), datagen.GenRandomByteArray(r, 32))
		tx.AddTxIn(
			wire.NewTxIn(wire.NewOutPoint(inputHash, uint32(r.Intn(20))), nil, nil),
		)
		tx.AddTxOut(
			wire.NewTxOut(int64(r.Intn(1000)), datagen.GenRandomByteArray(r, 32)),
		)
		script := datagen.GenRandomByteArray(r, 150)

		sig, err := btcstaking.SignTxWithOneScriptSpendInputFromScript(
			tx,
			foundingOutput,
			pk,
			script,
		)

		require.NoError(t, err)

		err = btcstaking.VerifyTransactionSigWithOutput(
			tx,
			foundingOutput,
			script,
			pk.PubKey(),
			sig.Serialize(),
		)

		require.NoError(t, err)
	})
}

func FuzzGeneratingMultisigSignatureValidation(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// generate 2-5 staker keys
		numStakers := r.Intn(4) + 2 // 2-5 stakers
		stakerPrivKeys := make([]*btcec.PrivateKey, numStakers)
		stakerPubKeys := make([]*btcec.PublicKey, numStakers)
		for i := 0; i < numStakers; i++ {
			pk, err := btcec.NewPrivateKey()
			require.NoError(t, err)
			stakerPrivKeys[i] = pk
			stakerPubKeys[i] = pk.PubKey()
		}

		// pick random M-of-N quorum (at least 1, at most N)
		stakerQuorum := r.Intn(numStakers) + 1

		inputHash, err := chainhash.NewHash(datagen.GenRandomByteArray(r, 32))
		require.NoError(t, err)

		tx := wire.NewMsgTx(2)
		foundingOutput := wire.NewTxOut(int64(r.Intn(1000)+1000), datagen.GenRandomByteArray(r, 32))
		tx.AddTxIn(
			wire.NewTxIn(wire.NewOutPoint(inputHash, uint32(r.Intn(20))), nil, nil),
		)
		tx.AddTxOut(
			wire.NewTxOut(int64(r.Intn(1000)+500), datagen.GenRandomByteArray(r, 32)),
		)
		script := datagen.GenRandomByteArray(r, 150)

		// sign with exactly M stakers (the quorum)
		pubKey2Sig := make(map[*btcec.PublicKey][]byte)
		for i := 0; i < stakerQuorum; i++ {
			sig, err := btcstaking.SignTxWithOneScriptSpendInputFromScript(
				tx,
				foundingOutput,
				stakerPrivKeys[i],
				script,
			)
			require.NoError(t, err)
			pubKey2Sig[stakerPubKeys[i]] = sig.Serialize()
		}

		// verify multisig
		err = btcstaking.VerifyTransactionMultiSigWithOutput(
			tx,
			foundingOutput,
			script,
			pubKey2Sig,
		)

		require.NoError(t, err)

		// test with wrong signature should fail
		wrongPk, err := btcec.NewPrivateKey()
		require.NoError(t, err)
		wrongSig, err := btcstaking.SignTxWithOneScriptSpendInputFromScript(
			tx,
			foundingOutput,
			wrongPk,
			script,
		)
		require.NoError(t, err)

		// replace one valid signature with wrong one
		if stakerQuorum > 0 {
			pubKey2SigWrong := make(map[*btcec.PublicKey][]byte)
			for k, v := range pubKey2Sig {
				pubKey2SigWrong[k] = v
			}
			pubKey2SigWrong[stakerPubKeys[0]] = wrongSig.Serialize()

			err = btcstaking.VerifyTransactionMultiSigWithOutput(
				tx,
				foundingOutput,
				script,
				pubKey2SigWrong,
			)
			require.Error(t, err)
			require.Contains(t, err.Error(), "signature")
		}
	})
}

func TestSlashingTxWithOverflowMustNotAccepted(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	// we do not care for inputs in staking tx
	stakingTx := wire.NewMsgTx(2)
	slashingLockTime := uint16(100)
	minStakingValue := 10000
	minFee := 2000
	slashingRate := sdkmath.LegacyNewDecWithPrec(1000, 4)
	sd := genValidStakingScriptData(t, r)

	info, err := btcstaking.BuildStakingInfo(
		sd.StakerKey,
		[]*btcec.PublicKey{sd.FinalityProviderKey},
		[]*btcec.PublicKey{sd.CovenantKey},
		1,
		sd.StakingTime,
		btcutil.Amount(r.Intn(5000)+minStakingValue),
		&chaincfg.MainNetParams,
	)

	require.NoError(t, err)
	stakingTx.AddTxOut(info.StakingOutput)
	bogusInputHashBytes := [32]byte{}
	bogusInputHash, _ := chainhash.NewHash(bogusInputHashBytes[:])
	stakingTx.AddTxIn(
		wire.NewTxIn(wire.NewOutPoint(bogusInputHash, 0), nil, nil),
	)

	slashingAddress, err := genRandomBTCAddress(r)
	require.NoError(t, err)

	slashingPkScript, err := txscript.PayToAddrScript(slashingAddress)
	require.NoError(t, err)

	// Construct slashing transaction using the provided parameters
	slashingTx, err := btcstaking.BuildSlashingTxFromStakingTxStrict(
		stakingTx,
		uint32(0),
		slashingPkScript,
		sd.StakerKey,
		slashingLockTime,
		int64(minFee),
		slashingRate,
		&chaincfg.MainNetParams,
	)
	require.NoError(t, err)
	require.NotNil(t, slashingTx)

	slashingTx.TxOut[0].Value = math.MaxInt64 / 8
	slashingTx.TxOut[1].Value = math.MaxInt64 / 8

	err = btcstaking.CheckSlashingTxMatchFundingTx(
		slashingTx,
		stakingTx,
		uint32(0),
		int64(minFee),
		slashingRate,
		slashingPkScript,
		sd.StakerKey,
		slashingLockTime,
		&chaincfg.MainNetParams,
	)
	require.Error(t, err)
	require.EqualError(t, err, "invalid slashing tx: btc transaction do not obey BTC rules: transaction output value is higher than max allowed value: 1152921504606846975 > 2.1e+15 ")
}

func TestNotAllowStakerKeyToBeFinalityProviderKey(t *testing.T) {
	r := rand.New(rand.NewSource(0))
	sd := genValidStakingScriptData(t, r)

	// Construct staking transaction using the provided parameters
	stakingTx, err := btcstaking.BuildStakingInfo(
		sd.StakerKey,
		[]*btcec.PublicKey{sd.StakerKey},
		[]*btcec.PublicKey{sd.CovenantKey},
		1,
		sd.StakingTime,
		btcutil.Amount(10000),
		&chaincfg.MainNetParams,
	)
	require.Nil(t, stakingTx)
	require.Error(t, err)
	require.True(t, errors.Is(err, btcstaking.ErrDuplicatedKeyInScript))

	unbondingTx, err := btcstaking.BuildUnbondingInfo(
		sd.StakerKey,
		[]*btcec.PublicKey{sd.StakerKey},
		[]*btcec.PublicKey{sd.CovenantKey},
		1,
		sd.StakingTime,
		btcutil.Amount(10000),
		&chaincfg.MainNetParams,
	)
	require.Nil(t, unbondingTx)
	require.Error(t, err)
	require.True(t, errors.Is(err, btcstaking.ErrDuplicatedKeyInScript))
}

func TestNotAllowStakerKeyToBeCovenantKey(t *testing.T) {
	r := rand.New(rand.NewSource(0))
	sd := genValidStakingScriptData(t, r)

	// Construct staking transaction using the provided parameters
	stakingTx, err := btcstaking.BuildStakingInfo(
		sd.StakerKey,
		[]*btcec.PublicKey{sd.FinalityProviderKey},
		[]*btcec.PublicKey{sd.StakerKey},
		1,
		sd.StakingTime,
		btcutil.Amount(10000),
		&chaincfg.MainNetParams,
	)
	require.Nil(t, stakingTx)
	require.Error(t, err)
	require.True(t, errors.Is(err, btcstaking.ErrDuplicatedKeyInScript))

	unbondingTx, err := btcstaking.BuildUnbondingInfo(
		sd.StakerKey,
		[]*btcec.PublicKey{sd.FinalityProviderKey},
		[]*btcec.PublicKey{sd.StakerKey},
		1,
		sd.StakingTime,
		btcutil.Amount(10000),
		&chaincfg.MainNetParams,
	)
	require.Nil(t, unbondingTx)
	require.Error(t, err)
	require.True(t, errors.Is(err, btcstaking.ErrDuplicatedKeyInScript))
}

func TestNotAllowFinalityProviderKeysAsCovenantKeys(t *testing.T) {
	r := rand.New(rand.NewSource(0))
	sd := genValidStakingScriptData(t, r)

	// Construct staking transaction using the provided parameters
	stakingTx, err := btcstaking.BuildStakingInfo(
		sd.StakerKey,
		[]*btcec.PublicKey{sd.FinalityProviderKey},
		[]*btcec.PublicKey{sd.FinalityProviderKey},
		1,
		sd.StakingTime,
		btcutil.Amount(10000),
		&chaincfg.MainNetParams,
	)
	require.Nil(t, stakingTx)
	require.Error(t, err)
	require.True(t, errors.Is(err, btcstaking.ErrDuplicatedKeyInScript))

	unbondingTx, err := btcstaking.BuildUnbondingInfo(
		sd.StakerKey,
		[]*btcec.PublicKey{sd.FinalityProviderKey},
		[]*btcec.PublicKey{sd.FinalityProviderKey},
		1,
		sd.StakingTime,
		btcutil.Amount(10000),
		&chaincfg.MainNetParams,
	)
	require.Nil(t, unbondingTx)
	require.Error(t, err)
	require.True(t, errors.Is(err, btcstaking.ErrDuplicatedKeyInScript))
}

func TestCheckPreSignedTxSanity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		genTx          func() *wire.MsgTx
		numInputs      uint32
		numOutputs     uint32
		minTxVersion   int32
		maxTxVersion   int32
		wantErr        bool
		expectedErrMsg string
	}{
		{
			name: "valid tx",
			genTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx(2)
				tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil, nil))
				tx.AddTxOut(wire.NewTxOut(1000, nil))
				return tx
			},
			numInputs:    1,
			numOutputs:   1,
			minTxVersion: 1,
			maxTxVersion: 2,
			wantErr:      false,
		},
		{
			name: "valid tx with required specific version 2",
			genTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx(2)
				tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil, nil))
				tx.AddTxOut(wire.NewTxOut(1000, nil))
				return tx
			},
			numInputs:    1,
			numOutputs:   1,
			minTxVersion: 2,
			maxTxVersion: 2,
			wantErr:      false,
		},
		{
			name: "invalid tx when requireing specific version 2",
			genTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx(3)
				tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil, nil))
				tx.AddTxOut(wire.NewTxOut(1000, nil))
				return tx
			},
			numInputs:      1,
			numOutputs:     1,
			minTxVersion:   2,
			maxTxVersion:   2,
			wantErr:        true,
			expectedErrMsg: "tx version must be between 2 and 2",
		},
		{
			name: "non standard version tx",
			genTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx(0)
				tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil, nil))
				tx.AddTxOut(wire.NewTxOut(1000, nil))
				return tx
			},
			numInputs:      1,
			numOutputs:     1,
			minTxVersion:   1,
			maxTxVersion:   2,
			wantErr:        true,
			expectedErrMsg: "tx version must be between 1 and 2",
		},
		{
			name: "transaction with locktime",
			genTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx(2)
				tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil, nil))
				tx.AddTxOut(wire.NewTxOut(1000, nil))
				tx.LockTime = 1
				return tx
			},
			numInputs:      1,
			numOutputs:     1,
			minTxVersion:   1,
			maxTxVersion:   2,
			wantErr:        true,
			expectedErrMsg: "pre-signed tx must not have locktime",
		},
		{
			name: "transaction with sig script",
			genTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx(2)
				tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil, nil))
				tx.AddTxOut(wire.NewTxOut(1000, nil))
				tx.TxIn[0].SignatureScript = []byte{0x01, 0x02, 0x03}
				return tx
			},
			numInputs:      1,
			numOutputs:     1,
			minTxVersion:   1,
			maxTxVersion:   2,
			wantErr:        true,
			expectedErrMsg: "pre-signed tx must not have signature script",
		},
		{
			name: "transaction with invalid amount of inputs",
			genTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx(2)
				tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil, nil))
				tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 1), nil, nil))
				tx.AddTxOut(wire.NewTxOut(1000, nil))
				return tx
			},
			numInputs:      1,
			numOutputs:     1,
			minTxVersion:   1,
			maxTxVersion:   2,
			wantErr:        true,
			expectedErrMsg: "tx must have exactly 1 inputs",
		},
		{
			name: "transaction with invalid amount of outputs",
			genTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx(2)
				tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil, nil))
				tx.AddTxOut(wire.NewTxOut(1000, nil))
				tx.AddTxOut(wire.NewTxOut(1000, nil))
				return tx
			},
			numInputs:      1,
			numOutputs:     1,
			minTxVersion:   1,
			maxTxVersion:   2,
			wantErr:        true,
			expectedErrMsg: "tx must have exactly 1 outputs",
		},
		{
			name: "replacable transaction",
			genTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx(2)
				tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil, nil))
				tx.AddTxOut(wire.NewTxOut(1000, nil))
				tx.TxIn[0].Sequence = wire.MaxTxInSequenceNum - 1
				return tx
			},
			numInputs:      1,
			numOutputs:     1,
			minTxVersion:   1,
			maxTxVersion:   2,
			wantErr:        true,
			expectedErrMsg: "pre-signed tx must not be replaceable",
		},
		{
			name: "transaction with too big witness",
			genTx: func() *wire.MsgTx {
				tx := wire.NewMsgTx(2)
				tx.AddTxIn(wire.NewTxIn(wire.NewOutPoint(&chainhash.Hash{}, 0), nil, nil))
				tx.AddTxOut(wire.NewTxOut(1000, nil))
				witness := [20000000]byte{}
				tx.TxIn[0].Witness = [][]byte{witness[:]}
				return tx
			},
			numInputs:      1,
			numOutputs:     1,
			minTxVersion:   1,
			maxTxVersion:   2,
			wantErr:        true,
			expectedErrMsg: "tx weight must not exceed 400000",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := btcstaking.CheckPreSignedTxSanity(
				tt.genTx(), tt.numInputs, tt.numOutputs, tt.minTxVersion, tt.maxTxVersion,
			)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAllowSlashingOutputToBeOPReturn(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// we do not care for inputs in staking tx
	stakingTx := wire.NewMsgTx(2)
	bogusInputHashBytes := [32]byte{}
	bogusInputHash, _ := chainhash.NewHash(bogusInputHashBytes[:])
	stakingTx.AddTxIn(
		wire.NewTxIn(wire.NewOutPoint(bogusInputHash, 0), nil, nil),
	)

	stakingOutputIdx := 0
	slashingLockTime := uint16(r.Intn(1000) + 1)
	sd := genValidStakingScriptData(t, r)

	minStakingValue := 500000
	minFee := 2000

	info, err := btcstaking.BuildStakingInfo(
		sd.StakerKey,
		[]*btcec.PublicKey{sd.FinalityProviderKey},
		[]*btcec.PublicKey{sd.CovenantKey},
		1,
		sd.StakingTime,
		btcutil.Amount(r.Intn(5000)+minStakingValue),
		&chaincfg.MainNetParams,
	)

	require.NoError(t, err)
	stakingTx.AddTxOut(info.StakingOutput)

	// super small slashing rate, slashing output wil be 50sats
	slashingRate := sdkmath.LegacyMustNewDecFromStr("0.0001")

	opReturnSlashingScript, err := txscript.NullDataScript([]byte("hello"))
	require.NoError(t, err)

	// Construct slashing transaction using the provided parameters
	slashingTx, err := btcstaking.BuildSlashingTxFromStakingTxStrict(
		stakingTx,
		uint32(stakingOutputIdx),
		opReturnSlashingScript,
		sd.StakerKey,
		slashingLockTime,
		int64(minFee),
		slashingRate,
		&chaincfg.MainNetParams,
	)
	require.NoError(t, err)
	require.NotNil(t, slashingTx)

	err = btcstaking.CheckSlashingTxMatchFundingTx(
		slashingTx,
		stakingTx,
		uint32(stakingOutputIdx),
		int64(minFee),
		slashingRate,
		opReturnSlashingScript,
		sd.StakerKey,
		slashingLockTime,
		&chaincfg.MainNetParams,
	)
	require.NoError(t, err)
}
