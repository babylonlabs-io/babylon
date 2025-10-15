package types_test

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/cometbft/cometbft/crypto/tmhash"
	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	addrStr1 := datagen.GenRandomAddress().String()
	addrStr2 := datagen.GenRandomAddress().String()
	hashStr := datagen.GenRandomHexStr(r, 32)
	msgHash := types.HashMsg(&types.MsgWithdrawReward{
		Address: "address",
	})
	validMsgHash := hex.EncodeToString(msgHash)
	badHashStr := datagen.GenRandomHexStr(r, 34)
	height := datagen.RandomInt(r, 100) + 5
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
					{StakeholderType: types.COSTAKER, Address: addrStr1, RewardGauge: datagen.GenRandomRewardGauge(r)},
				},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
				EventRewardTracker:                 []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:                     []types.FPDirectGaugeEntry{},
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
				EventRewardTracker:                 []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:                     []types.FPDirectGaugeEntry{},
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
				EventRewardTracker:                 []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:                     []types.FPDirectGaugeEntry{},
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
				EventRewardTracker:                 []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:                     []types.FPDirectGaugeEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("invalid withdraw addresses: duplicate entry for key: %s", addrStr1),
		},
		{
			desc: "Genesis valid MsgHashes",
			genState: &types.GenesisState{
				Params:                             types.DefaultParams(),
				BtcStakingGauges:                   []types.BTCStakingGaugeEntry{},
				RewardGauges:                       []types.RewardGaugeEntry{},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{validMsgHash},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
				EventRewardTracker:                 []types.EventsPowerUpdateAtHeightEntry{},
			},
			valid:  true,
			errMsg: "",
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
				EventRewardTracker:                 []types.EventsPowerUpdateAtHeightEntry{},
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
				EventRewardTracker:                 []types.EventsPowerUpdateAtHeightEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("duplicate hash: %s", hashStr),
		},
		{
			desc: "Genesis with bad hash len in MsgHashes",
			genState: &types.GenesisState{
				Params:                             types.DefaultParams(),
				BtcStakingGauges:                   []types.BTCStakingGaugeEntry{},
				RewardGauges:                       []types.RewardGaugeEntry{},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{badHashStr},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
				EventRewardTracker:                 []types.EventsPowerUpdateAtHeightEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("hash size should be %d characters: %s", tmhash.Size, badHashStr),
		},
		{
			desc: "Genesis with bad decoded msg in MsgHashes",
			genState: &types.GenesisState{
				Params:                             types.DefaultParams(),
				BtcStakingGauges:                   []types.BTCStakingGaugeEntry{},
				RewardGauges:                       []types.RewardGaugeEntry{},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{"ças"},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
				EventRewardTracker:                 []types.EventsPowerUpdateAtHeightEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("error decoding msg hash: %s", `encoding/hex: invalid byte: U+00C3 'Ã'`),
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
				EventRewardTracker:                 []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:                     []types.FPDirectGaugeEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("invalid finality providers current rewards: duplicate entry for key: %s", addrStr1),
		},
		{
			desc: "Genesis with 2 historical rewards for same finality provider and different period",
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
							r.Period = 3
							return &r
						}(),
					},
				},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{
					{
						Address: addrStr1,
						Period:  0,
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
				EventRewardTracker:           []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:               []types.FPDirectGaugeEntry{},
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
				EventRewardTracker:           []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:               []types.FPDirectGaugeEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("duplicate historical rewards for address: %s and period: 1", addrStr1),
		},
		{
			desc: "Genesis with valid BTC delegation rewards tracker but no delegator to fps data",
			genState: func() *types.GenesisState {
				fpAddr := datagen.GenRandomAccount().Address
				delAddr := datagen.GenRandomAccount().Address
				return &types.GenesisState{
					Params:              types.DefaultParams(),
					BtcStakingGauges:    []types.BTCStakingGaugeEntry{},
					RewardGauges:        []types.RewardGaugeEntry{},
					WithdrawAddresses:   []types.WithdrawAddressEntry{},
					RefundableMsgHashes: []string{},
					FinalityProvidersCurrentRewards: []types.FinalityProviderCurrentRewardsEntry{
						{
							Address: fpAddr,
							Rewards: func() *types.FinalityProviderCurrentRewards {
								r := datagen.GenRandomFinalityProviderCurrentRewards(r)
								r.Period = 2
								return &r
							}(),
						},
					},
					FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{
						{
							Address: fpAddr,
							Period:  0,
							Rewards: func() *types.FinalityProviderHistoricalRewards {
								v := datagen.GenRandomFPHistRwdWithDecimals(r)
								return &v
							}(),
						},
						{
							Address: fpAddr,
							Period:  1,
							Rewards: func() *types.FinalityProviderHistoricalRewards {
								v := datagen.GenRandomFPHistRwdWithDecimals(r)
								return &v
							}(),
						},
					},
					BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{
						{
							FinalityProviderAddress: fpAddr,
							DelegatorAddress:        delAddr,
							Tracker: func() *types.BTCDelegationRewardsTracker {
								v := datagen.GenRandomBTCDelegationRewardsTracker(r)
								v.StartPeriodCumulativeReward = 0
								return &v
							}(),
						},
					},
					BtcDelegatorsToFps: []types.BTCDelegatorToFpEntry{},
					EventRewardTracker: []types.EventsPowerUpdateAtHeightEntry{},
					FpDirectGauges:     []types.FPDirectGaugeEntry{},
				}
			}(),
			valid:  false,
			errMsg: "BTC delegators to finality providers map is not equal to the btc delegations data",
		},
		{
			desc: "Genesis with duplicated BTC delegation rewards tracker",
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
							r.Period = 2
							return &r
						}(),
					},
				},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{
					{
						Address: addrStr1,
						Period:  0,
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
				BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{
					{
						FinalityProviderAddress: addrStr1,
						DelegatorAddress:        addrStr2,
						Tracker: func() *types.BTCDelegationRewardsTracker {
							v := datagen.GenRandomBTCDelegationRewardsTracker(r)
							v.StartPeriodCumulativeReward = 0
							return &v
						}(),
					},
					{
						FinalityProviderAddress: addrStr1,
						DelegatorAddress:        addrStr2,
						Tracker: func() *types.BTCDelegationRewardsTracker {
							v := datagen.GenRandomBTCDelegationRewardsTracker(r)
							v.StartPeriodCumulativeReward = 0
							return &v
						}(),
					},
				},
				BtcDelegatorsToFps: []types.BTCDelegatorToFpEntry{{
					FinalityProviderAddress: addrStr1,
					DelegatorAddress:        addrStr2,
				}},
				EventRewardTracker: []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:     []types.FPDirectGaugeEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("duplicate btc delegation rewards tracker for finality provider: %s and delegator: %s", addrStr1, addrStr2),
		},
		{
			desc: "Genesis with duplicated BTC delegator to fp entry",
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
							r.Period = 2
							return &r
						}(),
					},
				},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{
					{
						Address: addrStr1,
						Period:  0,
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
				BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{
					{
						FinalityProviderAddress: addrStr1,
						DelegatorAddress:        addrStr2,
						Tracker: func() *types.BTCDelegationRewardsTracker {
							v := datagen.GenRandomBTCDelegationRewardsTracker(r)
							v.StartPeriodCumulativeReward = 0
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
				EventRewardTracker: []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:     []types.FPDirectGaugeEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("duplicate entry with finality provider: %s and delegator: %s", addrStr1, addrStr2),
		},
		{
			desc: "Genesis with duplicated block height entry",
			genState: &types.GenesisState{
				Params:                             types.DefaultParams(),
				BtcStakingGauges:                   []types.BTCStakingGaugeEntry{},
				RewardGauges:                       []types.RewardGaugeEntry{},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
				EventRewardTracker: []types.EventsPowerUpdateAtHeightEntry{
					{
						Height: height,
						Events: &types.EventsPowerUpdateAtHeight{},
					},
					{
						Height: height,
						Events: &types.EventsPowerUpdateAtHeight{},
					},
				},
				FpDirectGauges: []types.FPDirectGaugeEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("invalid events from reward tracker: duplicate entry for key: %d", height),
		},
		{
			desc: "Genesis with valid voting power update events",
			genState: &types.GenesisState{
				Params:                             types.DefaultParams(),
				BtcStakingGauges:                   []types.BTCStakingGaugeEntry{},
				RewardGauges:                       []types.RewardGaugeEntry{},
				WithdrawAddresses:                  []types.WithdrawAddressEntry{},
				RefundableMsgHashes:                []string{},
				FinalityProvidersCurrentRewards:    []types.FinalityProviderCurrentRewardsEntry{},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
				EventRewardTracker: []types.EventsPowerUpdateAtHeightEntry{
					{
						Height: height,
						Events: &types.EventsPowerUpdateAtHeight{
							Events: []*types.EventPowerUpdate{
								types.NewEventBtcDelegationActivated(addrStr1, addrStr2, datagen.RandomMathInt(r, 100).AddRaw(3000)),
								types.NewEventBtcDelegationUnboned(addrStr1, addrStr2, datagen.RandomMathInt(r, 100).AddRaw(1)),
							},
						},
					},
					{
						Height: height + 1,
						Events: &types.EventsPowerUpdateAtHeight{
							Events: []*types.EventPowerUpdate{
								types.NewEventBtcDelegationActivated(addrStr2, addrStr1, datagen.RandomMathInt(r, 100).AddRaw(1)),
							},
						},
					},
				},
				FpDirectGauges: []types.FPDirectGaugeEntry{},
			},
			valid: true,
		},
		{
			desc: "Genesis with FP current rewards but missing historical rewards",
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
							r.Period = 3
							return &r
						}(),
					},
				},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{},
				BtcDelegationRewardsTrackers:       []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:                 []types.BTCDelegatorToFpEntry{},
				EventRewardTracker:                 []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:                     []types.FPDirectGaugeEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("finality provider %s has current rewards with period 3 but no historical rewards", addrStr1),
		},
		{
			desc: "Genesis with FP missing some historical reward periods",
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
							r.Period = 3
							return &r
						}(),
					},
				},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{
					{
						Address: addrStr1,
						Period:  0,
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
				EventRewardTracker:           []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:               []types.FPDirectGaugeEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("finality provider %s is missing historical rewards for period 1 (current period is 3)", addrStr1),
		},
		{
			desc: "Genesis with FP historical rewards beyond current period",
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
							r.Period = 2
							return &r
						}(),
					},
				},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{
					{
						Address: addrStr1,
						Period:  0,
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
					{
						Address: addrStr1,
						Period:  3,
						Rewards: func() *types.FinalityProviderHistoricalRewards {
							v := datagen.GenRandomFPHistRwdWithDecimals(r)
							return &v
						}(),
					},
				},
				BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:           []types.BTCDelegatorToFpEntry{},
				EventRewardTracker:           []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:               []types.FPDirectGaugeEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("finality provider %s has historical rewards for period 3 which is >= current period 2", addrStr1),
		},
		{
			desc: "Genesis with FP historical rewards but no current rewards",
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
						Period:  0,
						Rewards: func() *types.FinalityProviderHistoricalRewards {
							v := datagen.GenRandomFPHistRwdWithDecimals(r)
							return &v
						}(),
					},
				},
				BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{},
				BtcDelegatorsToFps:           []types.BTCDelegatorToFpEntry{},
				EventRewardTracker:           []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:               []types.FPDirectGaugeEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("finality provider %s has historical rewards but no current rewards", addrStr1),
		},
		{
			desc: "Genesis with complete and valid FP rewards",
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
							r.Period = 3
							return &r
						}(),
					},
				},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{
					{
						Address: addrStr1,
						Period:  0,
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
				EventRewardTracker:           []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:               []types.FPDirectGaugeEntry{},
			},
			valid: true,
		},
		{
			desc: "Genesis with delegation tracker for FP without current rewards",
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
							v.StartPeriodCumulativeReward = 1
							return &v
						}(),
					},
				},
				BtcDelegatorsToFps: []types.BTCDelegatorToFpEntry{{
					FinalityProviderAddress: addrStr1,
					DelegatorAddress:        addrStr2,
				}},
				EventRewardTracker: []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:     []types.FPDirectGaugeEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("delegation tracker for finality provider %s exists but FP has no current rewards", addrStr1),
		},
		{
			desc: "Genesis with delegation tracker StartPeriodCumulativeReward >= FP current period",
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
							r.Period = 2
							return &r
						}(),
					},
				},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{
					{
						Address: addrStr1,
						Period:  0,
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
				BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{
					{
						FinalityProviderAddress: addrStr1,
						DelegatorAddress:        addrStr2,
						Tracker: func() *types.BTCDelegationRewardsTracker {
							v := datagen.GenRandomBTCDelegationRewardsTracker(r)
							v.StartPeriodCumulativeReward = 2
							return &v
						}(),
					},
				},
				BtcDelegatorsToFps: []types.BTCDelegatorToFpEntry{{
					FinalityProviderAddress: addrStr1,
					DelegatorAddress:        addrStr2,
				}},
				EventRewardTracker: []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:     []types.FPDirectGaugeEntry{},
			},
			valid:  false,
			errMsg: fmt.Sprintf("delegation tracker for FP %s and delegator %s has StartPeriodCumulativeReward 2 >= FP's current period 2", addrStr1, addrStr2),
		},
		{
			desc: "Genesis with valid delegation tracker",
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
							r.Period = 3
							return &r
						}(),
					},
				},
				FinalityProvidersHistoricalRewards: []types.FinalityProviderHistoricalRewardsEntry{
					{
						Address: addrStr1,
						Period:  0,
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
					{
						Address: addrStr1,
						Period:  2,
						Rewards: func() *types.FinalityProviderHistoricalRewards {
							v := datagen.GenRandomFPHistRwdWithDecimals(r)
							return &v
						}(),
					},
				},
				BtcDelegationRewardsTrackers: []types.BTCDelegationRewardsTrackerEntry{
					{
						FinalityProviderAddress: addrStr1,
						DelegatorAddress:        addrStr2,
						Tracker: func() *types.BTCDelegationRewardsTracker {
							v := datagen.GenRandomBTCDelegationRewardsTracker(r)
							v.StartPeriodCumulativeReward = 1
							return &v
						}(),
					},
				},
				BtcDelegatorsToFps: []types.BTCDelegatorToFpEntry{{
					FinalityProviderAddress: addrStr1,
					DelegatorAddress:        addrStr2,
				}},
				EventRewardTracker: []types.EventsPowerUpdateAtHeightEntry{},
				FpDirectGauges:     []types.FPDirectGaugeEntry{},
			},
			valid: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()

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

func TestFpDirectGauge_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	tests := []struct {
		name   string
		entry  types.FPDirectGaugeEntry
		expErr bool
		errMsg string
	}{
		{
			name:   "valid FP direct gauge",
			entry:  types.FPDirectGaugeEntry{Height: 100, Gauge: datagen.GenRandomGauge(r)},
			expErr: false,
		},
		{
			name: "nil gauge",
			entry: types.FPDirectGaugeEntry{
				Height: 100,
				Gauge:  nil,
			},
			expErr: true,
			errMsg: "FP direct gauge at height 100 has nil Gauge",
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
