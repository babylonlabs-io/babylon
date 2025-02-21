package types_test

import (
	"fmt"
	"math/rand"
	"testing"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbntypes "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stktypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestMsgCreateFinalityProviderValidateBasic(t *testing.T) {
	r := rand.New(rand.NewSource(10))
	randBigMoniker := datagen.GenRandomHexStr(r, 100)

	bigBtcPK := datagen.GenRandomByteArray(r, 100)

	fp, err := datagen.GenRandomFinalityProvider(r)
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
				Commission:  randomDecPointer(),
				Description: fpDesc,
				BtcPk:       []byte(*validPk),
			},
			expected: nil,
		},
		{
			name: "commission negative value",
			msg: &types.MsgEditFinalityProvider{
				Commission:  &negativeDec,
				Description: fpDesc,
				BtcPk:       []byte(*validPk),
			},
			expected: sdkerrors.ErrInvalidRequest.Wrap("commission rate must be between 0 and 1 (inclusive). Got negative value"),
		},
		{
			name: "commission greater than 1",
			msg: &types.MsgEditFinalityProvider{
				Commission:  &biggerThanOneDec,
				Description: fpDesc,
				BtcPk:       []byte(*validPk),
			},
			expected: types.ErrCommissionGTMaxRate,
		},
		{
			name: "empty description",
			msg: &types.MsgEditFinalityProvider{
				Description: nil,
				BtcPk:       []byte(*validPk),
			},
			expected: fmt.Errorf("empty description"),
		},
		{
			name: "empty moniker",
			msg: &types.MsgEditFinalityProvider{
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
				Description: fpDesc,
				BtcPk:       []byte("shortBTCpk"),
			},
			expected: fmt.Errorf("malformed BTC PK"),
		},
		{
			name: "invalid BTC public key (non-hex)",
			msg: &types.MsgEditFinalityProvider{
				Description: fpDesc,
				BtcPk:       []byte("B3C0F1D2E3A4B596C7D8E9FA1B2C3D4E5F6A7B8C9D0E1F2A3B4C5D6E7F8A9BZ"),
			},
			expected: fmt.Errorf("malformed BTC PK"),
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
