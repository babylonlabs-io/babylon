package types_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/x/incentive/types"
	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	addrStr := datagen.GenRandomAddress().String()
	tests := []struct {
		desc     string
		genState *types.GenesisState
		valid    bool
		errMsg   string
	}{
		{
			desc:     "default is valid",
			genState: types.DefaultGenesis(),
			valid:    true,
		},
		{
			desc: "Genesis with rewards gauges of same address and different types",
			genState: &types.GenesisState{
				Params:           types.DefaultParams(),
				BtcStakingGauges: []types.BTCStakingGaugeEntry{},
				RewardGauges: []types.RewardGaugeEntry{
					{StakeholderType: types.BTC_DELEGATION, Address: addrStr, RewardGauge: datagen.GenRandomRewardGauge(r)},
					{StakeholderType: types.FINALITY_PROVIDER, Address: addrStr, RewardGauge: datagen.GenRandomRewardGauge(r)},
				},
				WithdrawAddresses: []types.WithdrawAddressEntry{},
			},
			valid: true,
		},
		{
			desc: "Genesis with duplicated entries in rewards gauge",
			genState: &types.GenesisState{
				Params:           types.DefaultParams(),
				BtcStakingGauges: []types.BTCStakingGaugeEntry{},
				RewardGauges: []types.RewardGaugeEntry{
					{StakeholderType: types.BTC_DELEGATION, Address: addrStr, RewardGauge: datagen.GenRandomRewardGauge(r)},
					{StakeholderType: types.BTC_DELEGATION, Address: addrStr, RewardGauge: datagen.GenRandomRewardGauge(r)},
				},
				WithdrawAddresses: []types.WithdrawAddressEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("duplicate reward gauge for address: %s", addrStr),
		},
		{
			desc: "Genesis with duplicated entries in BTC staking gauge",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				BtcStakingGauges: []types.BTCStakingGaugeEntry{
					{Height: 100, Gauge: datagen.GenRandomGauge(r)},
					{Height: 100, Gauge: datagen.GenRandomGauge(r)},
				},
				RewardGauges:      []types.RewardGaugeEntry{},
				WithdrawAddresses: []types.WithdrawAddressEntry{},
			},
			valid:  false,
			errMsg: "duplicate BTC staking gauge for height: 100",
		},
		{
			desc: "Genesis with duplicated entries in withdraw addr",
			genState: &types.GenesisState{
				Params:           types.DefaultParams(),
				BtcStakingGauges: []types.BTCStakingGaugeEntry{},
				RewardGauges:     []types.RewardGaugeEntry{},
				WithdrawAddresses: []types.WithdrawAddressEntry{
					{
						DelegatorAddress: addrStr,
						WithdrawAddress:  datagen.GenRandomAddress().String(),
					},
					{
						DelegatorAddress: addrStr,
						WithdrawAddress:  datagen.GenRandomAddress().String(),
					},
				},
			},
			valid:  false,
			errMsg: fmt.Sprintf("duplicate delegator address: %s", addrStr),
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.genState.Validate()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
			}
		})
	}
}

func TestBTCStakingGaugeEntry_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	tests := []struct {
		name   string
		entry  types.BTCStakingGaugeEntry
		expErr bool
		errMsg string
	}{
		{
			name:   "valid BTC staking gauge",
			entry:  types.BTCStakingGaugeEntry{Height: 100, Gauge: datagen.GenRandomGauge(r)},
			expErr: false,
		},
		{
			name: "nil gauge",
			entry: types.BTCStakingGaugeEntry{
				Height: 100,
				Gauge:  nil,
			},
			expErr: true,
			errMsg: "BTC staking gauge at height 100 has nil Gauge",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.expErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRewardGaugeEntry_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	addr := datagen.GenRandomAddress().String()
	tests := []struct {
		name   string
		entry  types.RewardGaugeEntry
		expErr bool
		errMsg string
	}{
		{
			name: "valid reward gauge",
			entry: types.RewardGaugeEntry{
				Address:     datagen.GenRandomAddress().String(),
				RewardGauge: datagen.GenRandomRewardGauge(r),
			},
			expErr: false,
		},
		{
			name: "invalid reward gauge is nil",
			entry: types.RewardGaugeEntry{
				Address:     addr,
				RewardGauge: nil,
			},
			expErr: true,
			errMsg: fmt.Sprintf("reward gauge for address %s is nil", addr),
		},
		{
			name: "empty address",
			entry: types.RewardGaugeEntry{
				Address:     "",
				RewardGauge: datagen.GenRandomRewardGauge(r),
			},
			expErr: true,
			errMsg: "empty address",
		},
		{
			name: "invalid bech32 address",
			entry: types.RewardGaugeEntry{
				Address:     "cosmos1vlad9w6xq9adk5hdhjvrs8vsmhw3zcr99ml5v4",
				RewardGauge: datagen.GenRandomRewardGauge(r),
			},
			expErr: true,
			errMsg: "invalid address",
		},
		{
			name: "invalid bech32 address",
			entry: types.RewardGaugeEntry{
				Address:     "invalid_address",
				RewardGauge: datagen.GenRandomRewardGauge(r),
			},
			expErr: true,
			errMsg: "invalid address",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.expErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWithdrawAddressEntry_Validate(t *testing.T) {
	tests := []struct {
		name      string
		entry     types.WithdrawAddressEntry
		expectErr bool
		errMsg    string
	}{
		{
			name: "Valid addresses",
			entry: types.WithdrawAddressEntry{
				DelegatorAddress: datagen.GenRandomAddress().String(),
				WithdrawAddress:  datagen.GenRandomAddress().String(),
			},
			expectErr: false,
		},
		{
			name: "Invalid delegator address",
			entry: types.WithdrawAddressEntry{
				DelegatorAddress: "invalid_delegator",
				WithdrawAddress:  datagen.GenRandomAddress().String(),
			},
			expectErr: true,
			errMsg:    "invalid delegator",
		},
		{
			name: "Invalid withdraw address",
			entry: types.WithdrawAddressEntry{
				DelegatorAddress: datagen.GenRandomAddress().String(),
				WithdrawAddress:  "invalid_withdraw",
			},
			expectErr: true,
			errMsg:    "invalid withdrawer",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.expectErr {
				require.Error(t, err)
				require.ErrorContains(t, err, tc.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
