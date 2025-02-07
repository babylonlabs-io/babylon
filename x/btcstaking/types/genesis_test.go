package types_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"

	"github.com/babylonlabs-io/babylon/x/btcstaking/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	tests := []struct {
		desc     string
		genState func() *types.GenesisState
		valid    bool
	}{
		{
			desc:     "default is valid",
			genState: types.DefaultGenesis,
			valid:    true,
		},
		{
			desc: "valid genesis state",
			genState: func() *types.GenesisState {
				return &types.GenesisState{
					Params: []*types.Params{
						&types.Params{
							CovenantPks:          types.DefaultParams().CovenantPks,
							CovenantQuorum:       types.DefaultParams().CovenantQuorum,
							MinStakingValueSat:   10000,
							MaxStakingValueSat:   100000000,
							MinStakingTimeBlocks: 100,
							MaxStakingTimeBlocks: 1000,
							SlashingPkScript:     types.DefaultParams().SlashingPkScript,
							MinSlashingTxFeeSat:  500,
							MinCommissionRate:    sdkmath.LegacyMustNewDecFromStr("0.5"),
							SlashingRate:         sdkmath.LegacyMustNewDecFromStr("0.1"),
							UnbondingFeeSat:      types.DefaultParams().UnbondingFeeSat,
						},
					},
				}
			},
			valid: true,
		},
		{
			desc: "invalid slashing rate in genesis",
			genState: func() *types.GenesisState {
				return &types.GenesisState{
					Params: []*types.Params{
						&types.Params{
							CovenantPks:         types.DefaultParams().CovenantPks,
							CovenantQuorum:      types.DefaultParams().CovenantQuorum,
							SlashingPkScript:    types.DefaultParams().SlashingPkScript,
							MinSlashingTxFeeSat: 500,
							MinCommissionRate:   sdkmath.LegacyMustNewDecFromStr("0.5"),
							SlashingRate:        sdkmath.LegacyZeroDec(), // invalid slashing rate
							UnbondingFeeSat:     types.DefaultParams().UnbondingFeeSat,
						},
					},
				}
			},
			valid: false,
		},
		{
			desc: "min staking time larger than max staking time",
			genState: func() *types.GenesisState {
				d := types.DefaultGenesis()
				d.Params[0].MinStakingTimeBlocks = 1000
				d.Params[0].MaxStakingTimeBlocks = 100
				return d
			},
			valid: false,
		},
		{
			desc: "min staking value larger than max staking value",
			genState: func() *types.GenesisState {
				d := types.DefaultGenesis()
				d.Params[0].MinStakingValueSat = 1000
				d.Params[0].MaxStakingValueSat = 100
				return d
			},
			valid: false,
		},
		{
			desc: "parameters with btc activation height > 0 as initial params are valid",
			genState: func() *types.GenesisState {
				params1 := types.DefaultParams()
				params1.BtcActivationHeight = 100

				return &types.GenesisState{
					Params: []*types.Params{
						&params1,
					},
				}
			},
			valid: true,
		},
		{
			desc: "parameters with btc activation height not in ascending order are invalid",
			genState: func() *types.GenesisState {
				params1 := types.DefaultParams()
				params1.BtcActivationHeight = 100

				params2 := types.DefaultParams()
				params2.BtcActivationHeight = 101

				return &types.GenesisState{
					Params: []*types.Params{
						&params2,
						&params1,
					},
				}
			},
			valid: false,
		},
		{
			desc: "parameters with btc activation height in ascending order are valid",
			genState: func() *types.GenesisState {
				params1 := types.DefaultParams()
				params1.BtcActivationHeight = 100

				params2 := types.DefaultParams()
				params2.BtcActivationHeight = 101

				return &types.GenesisState{
					Params: []*types.Params{
						&params1,
						&params2,
					},
				}
			},
			valid: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			state := tc.genState()
			err := state.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}
