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
			desc: "default is valid",
			genState: func() *types.GenesisState {
				return types.DefaultGenesis()
			},
			valid: true,
		},
		{
			desc: "valid genesis state",
			genState: func() *types.GenesisState {
				return &types.GenesisState{
					Params: []*types.Params{
						&types.Params{
							CovenantPks:                types.DefaultParams().CovenantPks,
							CovenantQuorum:             types.DefaultParams().CovenantQuorum,
							MinStakingValueSat:         1000,
							MaxStakingValueSat:         100000000,
							MinStakingTimeBlocks:       100,
							MaxStakingTimeBlocks:       1000,
							SlashingPkScript:           types.DefaultParams().SlashingPkScript,
							MinSlashingTxFeeSat:        500,
							MinCommissionRate:          sdkmath.LegacyMustNewDecFromStr("0.5"),
							SlashingRate:               sdkmath.LegacyMustNewDecFromStr("0.1"),
							MaxActiveFinalityProviders: 100,
							UnbondingFeeSat:            types.DefaultParams().UnbondingFeeSat,
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
							CovenantPks:                types.DefaultParams().CovenantPks,
							CovenantQuorum:             types.DefaultParams().CovenantQuorum,
							SlashingPkScript:           types.DefaultParams().SlashingPkScript,
							MinSlashingTxFeeSat:        500,
							MinCommissionRate:          sdkmath.LegacyMustNewDecFromStr("0.5"),
							SlashingRate:               sdkmath.LegacyZeroDec(), // invalid slashing rate
							MaxActiveFinalityProviders: 100,
							UnbondingFeeSat:            types.DefaultParams().UnbondingFeeSat,
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
