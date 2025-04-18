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
	addrStr1 := datagen.GenRandomAddress().String()
	addrStr2 := datagen.GenRandomAddress().String()
	hashStr := datagen.GenRandomHexStr(r, 32)
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
					{StakeholderType: types.BTC_STAKER, Address: addrStr1, RewardGauge: datagen.GenRandomRewardGauge(r)},
					{StakeholderType: types.FINALITY_PROVIDER, Address: addrStr1, RewardGauge: datagen.GenRandomRewardGauge(r)},
				},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
			},
			valid: true,
		},
		{
			desc: "Genesis with duplicated entries in rewards gauge",
			genState: &types.GenesisState{
				Params:           types.DefaultParams(),
				BtcStakingGauges: []types.BTCStakingGaugeEntry{},
				RewardGauges: []types.RewardGaugeEntry{
					{StakeholderType: types.BTC_STAKER, Address: addrStr1, RewardGauge: datagen.GenRandomRewardGauge(r)},
					{StakeholderType: types.BTC_STAKER, Address: addrStr1, RewardGauge: datagen.GenRandomRewardGauge(r)},
				},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("duplicate reward gauge for address: %s", addrStr1),
		},
		{
			desc: "Genesis with duplicated entries in BTC staking gauge",
			genState: &types.GenesisState{
				Params: types.DefaultParams(),
				BtcStakingGauges: []types.BTCStakingGaugeEntry{
					{Height: 100, Gauge: datagen.GenRandomGauge(r)},
					{Height: 100, Gauge: datagen.GenRandomGauge(r)},
				},
				RewardGauges:                       []types.RewardGaugeEntry{},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
			},
			valid:  false,
			errMsg: "invalid BTC staking gauges: duplicate entry for key: 100",
		},
		{
			desc: "Genesis with duplicated entries in withdraw addr",
			genState: &types.GenesisState{
				Params:           types.DefaultParams(),
				BtcStakingGauges: []types.BTCStakingGaugeEntry{},
				RewardGauges:     []types.RewardGaugeEntry{},
				WithdrawAddresses: []types.WithdrawAddressEntry{
					{
						DelegatorAddress: addrStr1,
						WithdrawAddress:  datagen.GenRandomAddress().String(),
					},
					{
						DelegatorAddress: addrStr1,
						WithdrawAddress:  datagen.GenRandomAddress().String(),
					},
				},
				RefundableMsgHashes:                []string{},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("invalid withdraw addresses: duplicate entry for key: %s", addrStr1),
		},
		{
			desc: "Genesis with empty string in MsgHashes",
			genState: &types.GenesisState{
				Params:                             types.DefaultParams(),
				BtcStakingGauges:                   []types.BTCStakingGaugeEntry{},
				RewardGauges:                       []types.RewardGaugeEntry{},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{""},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
			},
			valid:  false,
			errMsg: "empty hash",
		},
		{
			desc: "Genesis with duplicate hash in MsgHashes",
			genState: &types.GenesisState{
				Params:                             types.DefaultParams(),
				BtcStakingGauges:                   []types.BTCStakingGaugeEntry{},
				RewardGauges:                       []types.RewardGaugeEntry{},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{hashStr, hashStr},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("duplicate hash: %s", hashStr),
		},
		{
			desc: "Genesis with 2 current rewards for same finality provider",
			genState: &types.GenesisState{
				Params:              types.DefaultParams(),
				BtcStakingGauges:    []types.BTCStakingGaugeEntry{},
				RewardGauges:        []types.RewardGaugeEntry{},
				WithdrawAddresses:   []types.WithdrawAddressEntry{},
				RefundableMsgHashes: []string{},
				FinalityProvidersCurrentRewards: []types.FinalityProviderCurrentRewardsEntry{
					{
						Address: addrStr1,
						Rewards: func() *types.FinalityProviderCurrentRewards {
							r := datagen.GenRandomFinalityProviderCurrentRewards(r)
							return &r
						}(),
					},
					{
						Address: addrStr1,
						Rewards: func() *types.FinalityProviderCurrentRewards {
							r := datagen.GenRandomFinalityProviderCurrentRewards(r)
							return &r
						}(),
					},
				},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("invalid finality providers current rewards: duplicate entry for key: %s", addrStr1),
		},
		{
			desc: "Genesis with 2 historical rewards for same finality provider and different period",
			genState: &types.GenesisState{
				Params:                          types.DefaultParams(),
				BtcStakingGauges:                []types.BTCStakingGaugeEntry{},
				RewardGauges:                    []types.RewardGaugeEntry{},
				WithdrawAddresses:               []types.WithdrawAddressEntry{},
				RefundableMsgHashes:             []string{},
				FinalityProvidersCurrentRewards: []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{
					{
						Address: addrStr1,
						Period:  1,
						Rewards: func() *types.FinalityProviderHistoricalRewards {
							v := datagen.GenRandomFPHistRwdWithDecimals(r)
							return &v
						}(),
					},
					{
						Address: addrStr1,
						Period:  2,
						Rewards: func() *types.FinalityProviderHistoricalRewards {
							v := datagen.GenRandomFPHistRwdWithDecimals(r)
							return &v
						}(),
					},
				},
				BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:           []types.BTCDelegatorToFpEntry{},
			},
			valid: true,
		},
		{
			desc: "Genesis with 2 historical rewards for same finality provider and same period",
			genState: &types.GenesisState{
				Params:                          types.DefaultParams(),
				BtcStakingGauges:                []types.BTCStakingGaugeEntry{},
				RewardGauges:                    []types.RewardGaugeEntry{},
				WithdrawAddresses:               []types.WithdrawAddressEntry{},
				RefundableMsgHashes:             []string{},
				FinalityProvidersCurrentRewards: []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{
					{
						Address: addrStr1,
						Period:  1,
						Rewards: func() *types.FinalityProviderHistoricalRewards {
							v := datagen.GenRandomFPHistRwdWithDecimals(r)
							return &v
						}(),
					},
					{
						Address: addrStr1,
						Period:  1,
						Rewards: func() *types.FinalityProviderHistoricalRewards {
							v := datagen.GenRandomFPHistRwdWithDecimals(r)
							return &v
						}(),
					},
				},
				BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:           []types.BTCDelegatorToFpEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("duplicate historical rewards for address: %s and period: 1", addrStr1),
		},
		{
			desc: "Genesis with valid BTC delegation rewards tracker but no delegator to fps data",
			genState: &types.GenesisState{
				Params:                             types.DefaultParams(),
				BtcStakingGauges:                   []types.BTCStakingGaugeEntry{},
				RewardGauges:                       []types.RewardGaugeEntry{},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{
					{
						FinalityProviderAddress: datagen.GenRandomAccount().Address,
						DelegatorAddress:        datagen.GenRandomAccount().Address,
						Tracker: func() *types.BTCDelegationRewardsTracker {
							v := datagen.GenRandomBTCDelegationRewardsTracker(r)
							return &v
						}(),
					},
				},
				BtcDelegatorsToFps: []types.BTCDelegatorToFpEntry{},
			},
			valid:  false,
			errMsg: "BTC delegators to finality providers map is not equal to the btc delegations data",
		},
		{
			desc: "Genesis with duplicated BTC delegation rewards tracker",
			genState: &types.GenesisState{
				Params:                             types.DefaultParams(),
				BtcStakingGauges:                   []types.BTCStakingGaugeEntry{},
				RewardGauges:                       []types.RewardGaugeEntry{},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{
					{
						FinalityProviderAddress: addrStr1,
						DelegatorAddress:        addrStr2,
						Tracker: func() *types.BTCDelegationRewardsTracker {
							v := datagen.GenRandomBTCDelegationRewardsTracker(r)
							return &v
						}(),
					},
					{
						FinalityProviderAddress: addrStr1,
						DelegatorAddress:        addrStr2,
						Tracker: func() *types.BTCDelegationRewardsTracker {
							v := datagen.GenRandomBTCDelegationRewardsTracker(r)
							return &v
						}(),
					},
				},
				BtcDelegatorsToFps: []types.BTCDelegatorToFpEntry{{
					FinalityProviderAddress: addrStr1,
					DelegatorAddress:        addrStr2,
				}},
			},
			valid:  false,
			errMsg: fmt.Sprintf("duplicate btc delegation rewards tracker for finality provider: %s and delegator: %s", addrStr1, addrStr2),
		},
		{
			desc: "Genesis with duplicated BTC delegator to fp entry",
			genState: &types.GenesisState{
				Params:                             types.DefaultParams(),
				BtcStakingGauges:                   []types.BTCStakingGaugeEntry{},
				RewardGauges:                       []types.RewardGaugeEntry{},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{
					{
						FinalityProviderAddress: addrStr1,
						DelegatorAddress:        addrStr2,
						Tracker: func() *types.BTCDelegationRewardsTracker {
							v := datagen.GenRandomBTCDelegationRewardsTracker(r)
							return &v
						}(),
					},
				},
				BtcDelegatorsToFps: []types.BTCDelegatorToFpEntry{
					{
						FinalityProviderAddress: addrStr1,
						DelegatorAddress:        addrStr2,
					},
					{
						FinalityProviderAddress: addrStr1,
						DelegatorAddress:        addrStr2,
					},
				},
			},
			valid:  false,
			errMsg: fmt.Sprintf("duplicate entry with finality provider: %s and delegator: %s", addrStr1, addrStr2),
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

func TestFinalityProviderCurrentRewardsEntry_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	rw := datagen.GenRandomFinalityProviderCurrentRewards(r)
	addrStr := datagen.GenRandomAccount().Address
	testCases := []struct {
		name   string
		entry  types.FinalityProviderCurrentRewardsEntry
		expErr bool
		errMsg string
	}{
		{
			name: "valid entry",
			entry: types.FinalityProviderCurrentRewardsEntry{
				Address: datagen.GenRandomAddress().String(),
				Rewards: &rw,
			},
			expErr: false,
		},
		{
			name:   "invalid address",
			entry:  types.FinalityProviderCurrentRewardsEntry{"invalid_address", &rw},
			expErr: true,
			errMsg: "invalid finality provider",
		},
		{
			name:   "nil rewards",
			entry:  types.FinalityProviderCurrentRewardsEntry{addrStr, nil},
			expErr: true,
			errMsg: fmt.Sprintf("current rewards for address %s is nil", addrStr),
		},
	}

	for _, tc := range testCases {
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
