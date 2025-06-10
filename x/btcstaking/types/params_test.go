package types_test

import (
	"testing"

	"cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/btcstaking"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/stretchr/testify/require"
)

func TestHeightToVersionMap(t *testing.T) {
	testCases := []struct {
		name          string
		heightToVer   types.HeightToVersionMap
		height        uint64
		expectedVer   uint32
		expectedError bool
	}{
		{
			name:          "empty map returns error",
			heightToVer:   types.HeightToVersionMap{},
			height:        100,
			expectedError: true,
		},
		{
			name: "exact height match for first pair",
			heightToVer: types.HeightToVersionMap{
				Pairs: []*types.HeightVersionPair{
					{
						StartHeight: 100,
						Version:     1,
					},
					{
						StartHeight: 200,
						Version:     2,
					},
				},
			},

			height:      100,
			expectedVer: 1,
		},
		{
			name: "exact height match for second pair",
			heightToVer: types.HeightToVersionMap{
				Pairs: []*types.HeightVersionPair{
					{
						StartHeight: 100,
						Version:     1,
					},
					{
						StartHeight: 200,
						Version:     2,
					},
				},
			},

			height:      200,
			expectedVer: 2,
		},
		{
			name: "height between versions",
			heightToVer: types.HeightToVersionMap{
				Pairs: []*types.HeightVersionPair{
					{
						StartHeight: 100,
						Version:     1,
					},
					{
						StartHeight: 200,
						Version:     2,
					},
				},
			},
			height:      150,
			expectedVer: 1,
		},
		{
			name: "height after last version",
			heightToVer: types.HeightToVersionMap{
				Pairs: []*types.HeightVersionPair{
					{
						StartHeight: 100,
						Version:     1,
					},
					{
						StartHeight: 200,
						Version:     2,
					},
				},
			},
			height:      300,
			expectedVer: 2,
		},
		{
			name: "height before first version",
			heightToVer: types.HeightToVersionMap{
				Pairs: []*types.HeightVersionPair{
					{
						StartHeight: 100,
						Version:     1,
					},
					{
						StartHeight: 200,
						Version:     2,
					},
				},
			},
			height:        99,
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			version, err := tc.heightToVer.GetVersionForHeight(tc.height)

			if tc.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedVer, version)
		})
	}
}

func TestParamsValidateNoDustSlashing(t *testing.T) {
	testCases := []struct {
		name          string
		modifyParams  func(p *types.Params)
		expectedError string
	}{
		{
			name: "valid non-dust output",
			modifyParams: func(p *types.Params) {
				p.MinStakingValueSat = 100000                       // 0.001 BTC
				p.UnbondingFeeSat = 1000                            // 0.00001 BTC
				p.SlashingRate = sdkmath.LegacyNewDecWithPrec(1, 1) // 0.1 (10%)
			},
			expectedError: "",
		},
		{
			name: "dust output due to small staking amount",
			modifyParams: func(p *types.Params) {
				p.MinStakingValueSat = 1000                         // 0.00001 BTC (very small)
				p.UnbondingFeeSat = 100                             // 0.000001 BTC
				p.SlashingRate = sdkmath.LegacyNewDecWithPrec(1, 1) // 0.1 (10%)
			},
			expectedError: "invalid parameters configuration. Minimum slashing output is dust",
		},
		{
			name: "OP_RETURN output is allowed to be dust",
			modifyParams: func(p *types.Params) {
				p.MinStakingValueSat = 1000                         // 0.00001 BTC (very small)
				p.UnbondingFeeSat = 100                             // 0.000001 BTC
				p.SlashingRate = sdkmath.LegacyNewDecWithPrec(1, 1) // 0.1 (10%)
				p.SlashingPkScript = []byte{txscript.OP_RETURN}
			},
			expectedError: "",
		},
		{
			name: "dust output due to low slashing rate",
			modifyParams: func(p *types.Params) {
				p.MinStakingValueSat = 500000                       // 0.001 BTC
				p.UnbondingFeeSat = 40000                           // 0.00001 BTC
				p.SlashingRate = sdkmath.LegacyNewDecWithPrec(1, 3) // 0.001 (0.1%)
			},
			expectedError: "invalid parameters configuration. Minimum slashing output is dust",
		},
		{
			// this test has similar parameters as the previous one but pk script is from
			// p2pwkh addrss which has higher dust threshold
			name: "pk script with witness has higher dust threshold",
			modifyParams: func(p *types.Params) {
				btcSk, err := btcec.NewPrivateKey()
				require.NoError(t, err)
				// Create P2WPKH address
				p2wpkhAddr, err := btcutil.NewAddressWitnessPubKeyHash(
					btcutil.Hash160(btcSk.PubKey().SerializeCompressed()),
					&chaincfg.MainNetParams,
				)
				require.NoError(t, err)

				// Get the pkScript for the P2WPKH address
				p.SlashingPkScript, err = txscript.PayToAddrScript(p2wpkhAddr)
				require.NoError(t, err)
				p.MinStakingValueSat = 500000                       // 0.001 BTC
				p.UnbondingFeeSat = 40000                           // 0.00001 BTC
				p.SlashingRate = sdkmath.LegacyNewDecWithPrec(1, 3) // 0.001 (0.1%)
			},
			expectedError: "",
		},
		{
			name: "negative UnbondingFeeSat",
			modifyParams: func(p *types.Params) {
				p.MinStakingValueSat = 100000                       // 0.001 BTC
				p.UnbondingFeeSat = -1                              // Incorrect value
				p.SlashingRate = sdkmath.LegacyNewDecWithPrec(1, 1) // 0.1 (10%)
			},
			expectedError: errors.Wrapf(btcstaking.ErrInvalidUnbondingFee, "(%d) is not a valid unbonding fee value", -1).Error(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := types.DefaultParams()
			tc.modifyParams(&params)

			err := params.Validate()

			if tc.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedError)
			}
		})
	}
}

func TestDefaultParamsAreValid(t *testing.T) {
	params := types.DefaultParams()
	require.NoError(t, params.Validate())
}
