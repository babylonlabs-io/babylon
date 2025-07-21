package types_test

import (
	"fmt"
	"math/rand"
	reflect "reflect"
	"testing"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	bbntypes "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/babylonlabs-io/babylon/v3/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestMsgCreateFinalityProviderValidateBasic(t *testing.T) {
	r := rand.New(rand.NewSource(10))
	randBigMoniker := datagen.GenRandomHexStr(r, 100)

	bigBtcPK := datagen.GenRandomByteArray(r, 100)

	fp, err := datagen.GenRandomFinalityProvider(r, "", "")
	require.NoError(t, err)

	invalidAddr := "bbnbadaddr"
	commission := types.NewCommissionRates(*fp.Commission, fp.CommissionInfo.MaxRate, fp.CommissionInfo.MaxChangeRate)

	tcs := []struct {
		title  string
		msg    *types.MsgCreateFinalityProvider
		expErr error
	}{
		{
			"valid: msg create fp",
			&types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: fp.Description,
				Commission:  commission,
				BtcPk:       fp.BtcPk,
				Pop:         fp.Pop,
			},
			nil,
		},
		{
			"invalid: empty commission rates",
			&types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: fp.Description,
				Commission:  types.CommissionRates{},
				BtcPk:       fp.BtcPk,
				Pop:         fp.Pop,
			},
			fmt.Errorf("empty commission"),
		},
		{
			"invalid: empty description",
			&types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: nil,
				Commission:  commission,
				BtcPk:       fp.BtcPk,
				Pop:         fp.Pop,
			},
			fmt.Errorf("empty description"),
		},
		{
			"invalid: empty moniker",
			&types.MsgCreateFinalityProvider{
				Addr: fp.Addr,
				Description: &stktypes.Description{
					Moniker:         "",
					Identity:        fp.Description.Identity,
					Website:         fp.Description.Website,
					SecurityContact: fp.Description.SecurityContact,
					Details:         fp.Description.Details,
				},
				Commission: commission,
				BtcPk:      fp.BtcPk,
				Pop:        fp.Pop,
			},
			fmt.Errorf("empty moniker"),
		},
		{
			"invalid: big moniker",
			&types.MsgCreateFinalityProvider{
				Addr: fp.Addr,
				Description: &stktypes.Description{
					Moniker:         randBigMoniker,
					Identity:        fp.Description.Identity,
					Website:         fp.Description.Website,
					SecurityContact: fp.Description.SecurityContact,
					Details:         fp.Description.Details,
				},
				Commission: commission,
				BtcPk:      fp.BtcPk,
				Pop:        fp.Pop,
			},
			errors.Wrapf(sdkerrors.ErrInvalidRequest, "invalid moniker length; got: %d, max: %d", len(randBigMoniker), stktypes.MaxMonikerLength),
		},
		{
			"invalid: empty BTC pk",
			&types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: fp.Description,
				Commission:  commission,
				BtcPk:       nil,
				Pop:         fp.Pop,
			},
			fmt.Errorf("empty BTC public key"),
		},
		{
			"invalid: invalid BTC pk",
			&types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: fp.Description,
				Commission:  commission,
				BtcPk:       (*bbntypes.BIP340PubKey)(&bigBtcPK),
				Pop:         fp.Pop,
			},
			fmt.Errorf("invalid BTC public key: %v", fmt.Errorf("bad pubkey byte string size (want %v, have %v)", 32, len(bigBtcPK))),
		},
		{
			"invalid: empty PoP",
			&types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: fp.Description,
				Commission:  commission,
				BtcPk:       fp.BtcPk,
				Pop:         nil,
			},
			fmt.Errorf("empty proof of possession"),
		},
		{
			"invalid: empty PoP",
			&types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: fp.Description,
				Commission:  commission,
				BtcPk:       fp.BtcPk,
				Pop:         nil,
			},
			fmt.Errorf("empty proof of possession"),
		},
		{
			"invalid: bad addr",
			&types.MsgCreateFinalityProvider{
				Addr:        invalidAddr,
				Description: fp.Description,
				Commission:  commission,
				BtcPk:       fp.BtcPk,
				Pop:         fp.Pop,
			},
			fmt.Errorf("invalid FP addr: %s - %v", invalidAddr, fmt.Errorf("decoding bech32 failed: invalid separator index -1")),
		},
		{
			"invalid: bad PoP empty sig",
			&types.MsgCreateFinalityProvider{
				Addr:        fp.Addr,
				Description: fp.Description,
				Commission:  commission,
				BtcPk:       fp.BtcPk,
				Pop: &types.ProofOfPossessionBTC{
					BtcSig: nil,
				},
			},
			fmt.Errorf("empty BTC signature"),
		},
	}

	for _, tc := range tcs {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			actErr := tc.msg.ValidateBasic()
			if tc.expErr != nil {
				require.EqualError(t, actErr, tc.expErr.Error())
				return
			}
			require.NoError(t, actErr)
		})
	}
}

func TestMsgEditFinalityProviderValidateBasic(t *testing.T) {
	var (
		r                = rand.New(rand.NewSource(10))
		addr             = datagen.GenRandomAddress().String()
		randomDecPointer = func() *math.LegacyDec {
			val := datagen.RandomLegacyDec(r, 10, 1)
			return &val
		}
		negativeDec      = math.LegacyNewDecWithPrec(-1, 2)
		biggerThanOneDec = math.LegacyOneDec().Add(math.LegacyOneDec())
		fpDesc           = &stktypes.Description{
			Moniker: "test description",
		}
	)
	validPk, err := datagen.GenRandomBIP340PubKey(r)
	require.NoError(t, err)
	testCases := []struct {
		name     string
		msg      *types.MsgEditFinalityProvider
		expected error
	}{
		{
			name: "valid commission and description",
			msg: &types.MsgEditFinalityProvider{
				Addr:        addr,
				Commission:  randomDecPointer(),
				Description: fpDesc,
				BtcPk:       []byte(*validPk),
			},
			expected: nil,
		},
		{
			name: "commission negative value",
			msg: &types.MsgEditFinalityProvider{
				Addr:        addr,
				Commission:  &negativeDec,
				Description: fpDesc,
				BtcPk:       []byte(*validPk),
			},
			expected: sdkerrors.ErrInvalidRequest.Wrap("commission rate must be between 0 and 1 (inclusive). Got negative value"),
		},
		{
			name: "commission greater than 1",
			msg: &types.MsgEditFinalityProvider{
				Addr:        addr,
				Commission:  &biggerThanOneDec,
				Description: fpDesc,
				BtcPk:       []byte(*validPk),
			},
			expected: types.ErrCommissionGTMaxRate,
		},
		{
			name: "empty description",
			msg: &types.MsgEditFinalityProvider{
				Addr:        addr,
				Description: nil,
				BtcPk:       []byte(*validPk),
			},
			expected: fmt.Errorf("empty description"),
		},
		{
			name: "empty moniker",
			msg: &types.MsgEditFinalityProvider{
				Addr: addr,
				Description: &stktypes.Description{
					Moniker: "",
				},
				BtcPk: []byte(*validPk),
			},
			expected: fmt.Errorf("empty moniker"),
		},
		{
			name: "invalid BTC public key length",
			msg: &types.MsgEditFinalityProvider{
				Addr:        addr,
				Description: fpDesc,
				BtcPk:       []byte("shortBTCpk"),
			},
			expected: fmt.Errorf("malformed BTC PK"),
		},
		{
			name: "invalid BTC public key (non-hex)",
			msg: &types.MsgEditFinalityProvider{
				Addr:        addr,
				Description: fpDesc,
				BtcPk:       []byte("B3C0F1D2E3A4B596C7D8E9FA1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9BZ"),
			},
			expected: fmt.Errorf("malformed BTC PK"),
		},
		{
			name: "empty FP addr",
			msg: &types.MsgEditFinalityProvider{
				Commission:  randomDecPointer(),
				Description: fpDesc,
				BtcPk:       []byte(*validPk),
			},
			expected: fmt.Errorf("invalid FP addr:  - empty address string is not allowed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expected != nil {
				require.EqualError(t, err, tc.expected.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMsgSelectiveSlashingEvidence_ValidateBasic(t *testing.T) {
	validAddr := datagen.GenRandomAddress().String()
	validSk := make([]byte, btcec.PrivKeyBytesLen) // 32 bytes

	testCases := []struct {
		name   string
		msg    types.MsgSelectiveSlashingEvidence
		expErr string
	}{
		{
			name: "valid message",
			msg: types.MsgSelectiveSlashingEvidence{
				Signer:           validAddr,
				RecoveredFpBtcSk: validSk,
			},
		},
		{
			name: "invalid signer address",
			msg: types.MsgSelectiveSlashingEvidence{
				Signer:           "not_bech32",
				RecoveredFpBtcSk: validSk,
			},
			expErr: "invalid signer addr",
		},
		{
			name: "invalid staking tx hash length",
			msg: types.MsgSelectiveSlashingEvidence{
				Signer:           validAddr,
				RecoveredFpBtcSk: validSk,
			},
			expErr: fmt.Sprintf("staking tx hash is not %d", chainhash.MaxHashStringSize),
		},
		{
			name: "invalid BTC SK length",
			msg: types.MsgSelectiveSlashingEvidence{
				Signer:           validAddr,
				RecoveredFpBtcSk: make([]byte, 16), // too short
			},
			expErr: fmt.Sprintf("malformed BTC SK. Expected length: %d", btcec.PrivKeyBytesLen),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.expErr)
		})
	}
}

func TestStructFieldConsistency(t *testing.T) {
	createType := reflect.TypeOf(types.MsgCreateBTCDelegation{})
	expandType := reflect.TypeOf(types.MsgBtcStakeExpand{})

	// Forward check: all fields in MsgCreateBTCDelegation are in MsgBtcStakeExpand
	// except StakingTxInclusionProof which was removed from MsgBtcStakeExpand
	var missingFromExpand []string
	for i := 0; i < createType.NumField(); i++ {
		createField := createType.Field(i)
		// Skip StakingTxInclusionProof field as it was intentionally removed from MsgBtcStakeExpand
		if createField.Name == "StakingTxInclusionProof" {
			continue
		}
		expandField, ok := expandType.FieldByName(createField.Name)
		if !ok {
			missingFromExpand = append(missingFromExpand, createField.Name)
			continue
		}
		if createField.Type != expandField.Type {
			t.Errorf("Field %s has different type in MsgBtcStakeExpand: %v != %v",
				createField.Name, createField.Type, expandField.Type)
		}
	}

	// Reverse check: all fields in MsgBtcStakeExpand (except last two: PreviousStakingTxHash and FundingTx) must be in MsgCreateBTCDelegation
	var missingFromCreate []string
	for i := 0; i < expandType.NumField()-2; i++ {
		expandField := expandType.Field(i)
		createField, ok := createType.FieldByName(expandField.Name)
		if !ok {
			missingFromCreate = append(missingFromCreate, expandField.Name)
			continue
		}
		if expandField.Type != createField.Type {
			t.Errorf("Field %s has different type in MsgCreateBTCDelegation: %v != %v",
				expandField.Name, expandField.Type, createField.Type)
		}
	}

	if len(missingFromExpand) > 0 {
		t.Errorf("MsgBtcStakeExpand is missing fields from MsgCreateBTCDelegation: %v", missingFromExpand)
	}
	if len(missingFromCreate) > 0 {
		t.Errorf("MsgCreateBTCDelegation is missing fields (except final 2) from MsgBtcStakeExpand: %v", missingFromCreate)
	}
}

func TestMsgAddBsnRewardsValidateBasic(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	// Helper to create valid test data
	validAddr := datagen.GenRandomAddress().String()
	validBsnConsumerId := "consumer-123"
	validCoin := sdk.NewCoin("ubbn", math.NewInt(1000000))
	validTotalRewards := sdk.NewCoins(validCoin)

	// Create valid BTC public keys
	validBtcPk1, err := datagen.GenRandomBIP340PubKey(r)
	require.NoError(t, err)
	validBtcPk2, err := datagen.GenRandomBIP340PubKey(r)
	require.NoError(t, err)

	// Create valid ratios that sum to 1.0
	ratio1 := math.LegacyNewDecWithPrec(6, 1) // 0.6
	ratio2 := math.LegacyNewDecWithPrec(4, 1) // 0.4

	validFpRatios := []types.FpRatio{
		{
			BtcPk: validBtcPk1,
			Ratio: ratio1,
		},
		{
			BtcPk: validBtcPk2,
			Ratio: ratio2,
		},
	}

	testCases := []struct {
		name     string
		msg      *types.MsgAddBsnRewards
		expected error
	}{
		{
			name: "valid message",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios:      validFpRatios,
			},
			expected: nil,
		},
		{
			name: "invalid sender address",
			msg: &types.MsgAddBsnRewards{
				Sender:        "invalid_address",
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios:      validFpRatios,
			},
			expected: fmt.Errorf("invalid sender address: invalid_address - decoding bech32 failed: invalid separator index -1"),
		},
		{
			name: "empty sender address",
			msg: &types.MsgAddBsnRewards{
				Sender:        "",
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios:      validFpRatios,
			},
			expected: fmt.Errorf("invalid sender address:  - empty address string is not allowed"),
		},
		{
			name: "empty BSN consumer ID",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: "",
				TotalRewards:  validTotalRewards,
				FpRatios:      validFpRatios,
			},
			expected: fmt.Errorf("empty BSN consumer ID"),
		},
		{
			name: "empty total rewards",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  sdk.NewCoins(),
				FpRatios:      validFpRatios,
			},
			expected: fmt.Errorf("empty total rewards"),
		},
		{
			name: "zero total rewards",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  sdk.NewCoins(sdk.NewCoin("ubbn", math.NewInt(0))),
				FpRatios:      validFpRatios,
			},
			expected: fmt.Errorf("empty total rewards"),
		},
		{
			name: "negative total rewards",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  sdk.Coins{sdk.Coin{Denom: "ubbn", Amount: math.NewInt(-100)}},
				FpRatios:      validFpRatios,
			},
			expected: fmt.Errorf("invalid total rewards: coin -100ubbn amount is not positive"),
		},
		{
			name: "invalid coin denomination",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  sdk.Coins{sdk.Coin{Denom: "", Amount: math.NewInt(1000)}},
				FpRatios:      validFpRatios,
			},
			expected: fmt.Errorf("invalid total rewards: invalid denom: "),
		},
		{
			name: "empty FP ratios",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios:      []types.FpRatio{},
			},
			expected: fmt.Errorf("empty finality provider ratios"),
		},
		{
			name: "nil BTC public key",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios: []types.FpRatio{
					{
						BtcPk: nil,
						Ratio: math.LegacyOneDec(),
					},
				},
			},
			expected: fmt.Errorf("finality provider 0: BTC public key cannot be nil"),
		},
		{
			name: "negative ratio",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios: []types.FpRatio{
					{
						BtcPk: validBtcPk1,
						Ratio: math.LegacyNewDecWithPrec(-1, 1), // -0.1
					},
				},
			},
			expected: fmt.Errorf("finality provider 0: ratio cannot be negative"),
		},
		{
			name: "ratio greater than 1.0",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios: []types.FpRatio{
					{
						BtcPk: validBtcPk1,
						Ratio: math.LegacyNewDecWithPrec(15, 1), // 1.5
					},
				},
			},
			expected: fmt.Errorf("finality provider 0: ratio cannot be greater than 1.0"),
		},
		{
			name: "zero ratio",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios: []types.FpRatio{
					{
						BtcPk: validBtcPk1,
						Ratio: math.LegacyZeroDec(),
					},
				},
			},
			expected: fmt.Errorf("finality provider 0: ratio cannot be zero"),
		},
		{
			name: "ratios sum to more than 1.0",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios: []types.FpRatio{
					{
						BtcPk: validBtcPk1,
						Ratio: math.LegacyNewDecWithPrec(7, 1), // 0.7
					},
					{
						BtcPk: validBtcPk2,
						Ratio: math.LegacyNewDecWithPrec(5, 1), // 0.5
					},
				},
			},
			expected: fmt.Errorf("finality provider ratios must sum to 1.0, got 1.200000000000000000"),
		},
		{
			name: "ratios sum to less than 1.0",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios: []types.FpRatio{
					{
						BtcPk: validBtcPk1,
						Ratio: math.LegacyNewDecWithPrec(3, 1), // 0.3
					},
					{
						BtcPk: validBtcPk2,
						Ratio: math.LegacyNewDecWithPrec(3, 1), // 0.3
					},
				},
			},
			expected: fmt.Errorf("finality provider ratios must sum to 1.0, got 0.600000000000000000"),
		},
		{
			name: "ratios sum to exactly 1.0 (edge case with precision)",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios: []types.FpRatio{
					{
						BtcPk: validBtcPk1,
						Ratio: math.LegacyNewDecWithPrec(3333333333, 10), // 0.3333333333
					},
					{
						BtcPk: validBtcPk2,
						Ratio: math.LegacyNewDecWithPrec(6666666667, 10), // 0.6666666667
					},
				},
			},
			expected: nil, // Should pass due to tolerance
		},
		{
			name: "single FP with ratio 1.0",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios: []types.FpRatio{
					{
						BtcPk: validBtcPk1,
						Ratio: math.LegacyOneDec(),
					},
				},
			},
			expected: nil,
		},
		{
			name: "multiple coins in total rewards",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards: sdk.NewCoins(
					sdk.NewCoin("ubbn", math.NewInt(1000000)),
					sdk.NewCoin("uatom", math.NewInt(500000)),
				),
				FpRatios: validFpRatios,
			},
			expected: nil,
		},
		{
			name: "duplicate finality provider BTC public keys",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios: []types.FpRatio{
					{
						BtcPk: validBtcPk1,
						Ratio: math.LegacyNewDecWithPrec(5, 1), // 0.5
					},
					{
						BtcPk: validBtcPk1, // Same BTC public key as above
						Ratio: math.LegacyNewDecWithPrec(5, 1), // 0.5
					},
				},
			},
			expected: fmt.Errorf("duplicate finality provider BTC public key at index 1: %s", validBtcPk1.MarshalHex()),
		},
		{
			name: "duplicate finality provider with three FPs (duplicate at index 2)",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards:  validTotalRewards,
				FpRatios: []types.FpRatio{
					{
						BtcPk: validBtcPk1,
						Ratio: math.LegacyNewDecWithPrec(4, 1), // 0.4
					},
					{
						BtcPk: validBtcPk2,
						Ratio: math.LegacyNewDecWithPrec(3, 1), // 0.3
					},
					{
						BtcPk: validBtcPk1, // Duplicate of first FP
						Ratio: math.LegacyNewDecWithPrec(3, 1), // 0.3
					},
				},
			},
			expected: fmt.Errorf("duplicate finality provider BTC public key at index 2: %s", validBtcPk1.MarshalHex()),
		},
		{
			name: "rewards with different denominations",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards: sdk.NewCoins(
					sdk.NewCoin("ubbn", math.NewInt(1000000)),
					sdk.NewCoin("uatom", math.NewInt(500000)),
					sdk.NewCoin("ustake", math.NewInt(250000)),
				),
				FpRatios: validFpRatios,
			},
			expected: nil,
		},
		{
			name: "rewards with mixed valid and invalid denominations",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards: sdk.Coins{
					sdk.NewCoin("ubbn", math.NewInt(1000000)),
					sdk.Coin{Denom: "invalid-denom!", Amount: math.NewInt(500000)}, // Invalid denom with special char
				},
				FpRatios: validFpRatios,
			},
			expected: fmt.Errorf("invalid total rewards: invalid denom: invalid-denom!"),
		},
		{
			name: "rewards with empty denomination",
			msg: &types.MsgAddBsnRewards{
				Sender:        validAddr,
				BsnConsumerId: validBsnConsumerId,
				TotalRewards: sdk.Coins{
					sdk.NewCoin("ubbn", math.NewInt(1000000)),
					sdk.Coin{Denom: "", Amount: math.NewInt(500000)},
				},
				FpRatios: validFpRatios,
			},
			expected: fmt.Errorf("invalid total rewards: invalid denom: "),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.msg.ValidateBasic()
			if tc.expected != nil {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expected.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestFpRatioValidateBasic(t *testing.T) {
	r := rand.New(rand.NewSource(42))

	validBtcPk, err := datagen.GenRandomBIP340PubKey(r)
	require.NoError(t, err)

	testCases := []struct {
		name     string
		fpRatio  *types.FpRatio
		expected error
	}{
		{
			name: "valid FpRatio",
			fpRatio: &types.FpRatio{
				BtcPk: validBtcPk,
				Ratio: math.LegacyNewDecWithPrec(5, 1), // 0.5
			},
			expected: nil,
		},
		{
			name: "valid FpRatio with ratio 1.0",
			fpRatio: &types.FpRatio{
				BtcPk: validBtcPk,
				Ratio: math.LegacyOneDec(),
			},
			expected: nil,
		},
		{
			name: "valid FpRatio with small ratio",
			fpRatio: &types.FpRatio{
				BtcPk: validBtcPk,
				Ratio: math.LegacyNewDecWithPrec(1, 10), // 0.0000000001
			},
			expected: nil,
		},
		{
			name: "nil BTC public key",
			fpRatio: &types.FpRatio{
				BtcPk: nil,
				Ratio: math.LegacyNewDecWithPrec(5, 1),
			},
			expected: fmt.Errorf("BTC public key cannot be nil"),
		},
		{
			name: "negative ratio",
			fpRatio: &types.FpRatio{
				BtcPk: validBtcPk,
				Ratio: math.LegacyNewDecWithPrec(-1, 1), // -0.1
			},
			expected: fmt.Errorf("ratio cannot be negative"),
		},
		{
			name: "zero ratio",
			fpRatio: &types.FpRatio{
				BtcPk: validBtcPk,
				Ratio: math.LegacyZeroDec(),
			},
			expected: fmt.Errorf("ratio cannot be zero"),
		},
		{
			name: "ratio greater than 1.0",
			fpRatio: &types.FpRatio{
				BtcPk: validBtcPk,
				Ratio: math.LegacyNewDecWithPrec(15, 1), // 1.5
			},
			expected: fmt.Errorf("ratio cannot be greater than 1.0"),
		},
		{
			name: "ratio much greater than 1.0",
			fpRatio: &types.FpRatio{
				BtcPk: validBtcPk,
				Ratio: math.LegacyNewDec(100), // 100.0
			},
			expected: fmt.Errorf("ratio cannot be greater than 1.0"),
		},
		{
			name: "invalid BTC public key length",
			fpRatio: &types.FpRatio{
				BtcPk: (*bbntypes.BIP340PubKey)(&[]byte{0x01, 0x02, 0x03}), // too short
				Ratio: math.LegacyNewDecWithPrec(5, 1),
			},
			expected: fmt.Errorf("invalid FP BTC PubKey. Expected length 32, got 3"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fpRatio.ValidateBasic()
			if tc.expected != nil {
				require.Error(t, err)
				require.Equal(t, tc.expected.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
