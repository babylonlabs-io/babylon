package types_test

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
	time "time"

	sdkmath "cosmossdk.io/math"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	bbntypes "github.com/babylonlabs-io/babylon/v4/types"
	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/chaincfg/chainhash"

	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	entriesCount := 10
	r := rand.New(rand.NewSource(time.Now().Unix()))
	txHashes := make([]string, 0, entriesCount)
	consumerEvents := make([]*types.ConsumerEvent, 0, entriesCount)
	for i := range entriesCount {
		txHash := datagen.GenRandomTx(r).TxHash()
		// hex encode the txHash bytes
		txHashStr := hex.EncodeToString(txHash[:])
		txHashes = append(txHashes, txHashStr)

		event := &types.ConsumerEvent{
			ConsumerId: fmt.Sprintf("consumer%d", i+1),
			Events: &types.BTCStakingIBCPacket{
				NewFp: []*types.NewFinalityProvider{{}},
			},
		}
		consumerEvents = append(consumerEvents, event)
	}

	tests := []struct {
		desc     string
		genState func() *types.GenesisState
		valid    bool
		errMsg   string
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
					AllowedStakingTxHashes: txHashes,
					ConsumerEvents:         consumerEvents,
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
			valid:  false,
			errMsg: "pairs must be sorted by start height in ascending order",
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
		{
			desc: "duplicate staking tx hash",
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
					AllowedStakingTxHashes: []string{txHashes[0], txHashes[0]},
				}
			},
			valid:  false,
			errMsg: "duplicate staking tx hash",
		},
		{
			desc: "duplicate consumer events",
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
					AllowedStakingTxHashes: txHashes,
					ConsumerEvents:         []*types.ConsumerEvent{consumerEvents[0], consumerEvents[0]},
				}
			},
			valid:  false,
			errMsg: "duplicate entry for key: consumer1",
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
				require.ErrorContains(t, err, tc.errMsg)
			}
		})
	}
}

func TestAllowedStakingTxHashStr_Validate(t *testing.T) {
	testCases := []struct {
		name   string
		input  types.AllowedStakingTxHashStr
		expErr bool
	}{
		{
			name:   "valid 32-byte hex string",
			input:  types.AllowedStakingTxHashStr("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			expErr: false,
		},
		{
			name:   "invalid hex string",
			input:  types.AllowedStakingTxHashStr("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"),
			expErr: true,
		},
		{
			name:   "too short (less than 32 bytes)",
			input:  types.AllowedStakingTxHashStr("abcd"),
			expErr: true,
		},
		{
			name:   "too long (more than 32 bytes)",
			input:  types.AllowedStakingTxHashStr("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			expErr: true,
		},
		{
			name:   "empty string",
			input:  types.AllowedStakingTxHashStr(""),
			expErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if (err != nil) != tc.expErr {
				t.Errorf("Validate() error = %v, expErr = %v", err, tc.expErr)
			}
		})
	}
}

func TestBTCDelegatorValidate(t *testing.T) {
	validLen := bbntypes.BIP340PubKeyLen
	validKey := bbntypes.BIP340PubKey(make([]byte, validLen))
	invalidKey := bbntypes.BIP340PubKey(make([]byte, validLen-1))
	validHash := chainhash.DoubleHashB([]byte("valid")) // 32-byte valid hash

	testCases := []struct {
		name      string
		delegator types.BTCDelegator
		expectErr string
	}{
		{
			name:      "nil FP BTC PubKey",
			delegator: types.BTCDelegator{},
			expectErr: "null FP BTC PubKey",
		},
		{
			name: "nil Delegator BTC PubKey",
			delegator: types.BTCDelegator{
				FpBtcPk: &validKey,
			},
			expectErr: "null Delegator BTC PubKey",
		},
		{
			name: "nil Index",
			delegator: types.BTCDelegator{
				FpBtcPk:  &validKey,
				DelBtcPk: &validKey,
			},
			expectErr: "null Index",
		},
		{
			name: "invalid FP BTC PubKey length",
			delegator: types.BTCDelegator{
				FpBtcPk:  &invalidKey,
				DelBtcPk: &validKey,
				Idx:      &types.BTCDelegatorDelegationIndex{StakingTxHashList: [][]byte{validHash}},
			},
			expectErr: fmt.Sprintf("invalid FP BTC PubKey. Expected length %d, got %d", validLen, validLen-1),
		},
		{
			name: "invalid Delegator BTC PubKey length",
			delegator: types.BTCDelegator{
				FpBtcPk:  &validKey,
				DelBtcPk: &invalidKey,
				Idx:      &types.BTCDelegatorDelegationIndex{StakingTxHashList: [][]byte{validHash}},
			},
			expectErr: fmt.Sprintf("invalid Delegator BTC PubKey. Expected length %d, got %d", validLen, validLen-1),
		},
		{
			name: "invalid hash in index",
			delegator: types.BTCDelegator{
				FpBtcPk:  &validKey,
				DelBtcPk: &validKey,
				Idx: &types.BTCDelegatorDelegationIndex{
					StakingTxHashList: [][]byte{[]byte("short hash")}, // invalid length for chainhash
				},
			},
			expectErr: "invalid hash length",
		},
		{
			name: "valid delegator",
			delegator: types.BTCDelegator{
				FpBtcPk:  &validKey,
				DelBtcPk: &validKey,
				Idx: &types.BTCDelegatorDelegationIndex{
					StakingTxHashList: [][]byte{validHash},
				},
			},
			expectErr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.delegator.Validate()
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectErr)
			}
		})
	}
}

func TestConsumerEventValidate(t *testing.T) {
	testCases := []struct {
		name      string
		event     types.ConsumerEvent
		expectErr string
	}{
		{
			name:      "empty Consumer ID",
			event:     types.ConsumerEvent{},
			expectErr: "empty Consumer ID",
		},
		{
			name: "nil Events",
			event: types.ConsumerEvent{
				ConsumerId: "consumer1",
				Events:     nil,
			},
			expectErr: "null Events",
		},
		{
			name: "empty Events fields",
			event: types.ConsumerEvent{
				ConsumerId: "consumer1",
				Events:     &types.BTCStakingIBCPacket{},
			},
			expectErr: "empty Events",
		},
		{
			name: "valid NewFp event",
			event: types.ConsumerEvent{
				ConsumerId: "consumer1",
				Events: &types.BTCStakingIBCPacket{
					NewFp: []*types.NewFinalityProvider{{}},
				},
			},
			expectErr: "",
		},
		{
			name: "valid ActiveDel event",
			event: types.ConsumerEvent{
				ConsumerId: "consumer1",
				Events: &types.BTCStakingIBCPacket{
					ActiveDel: []*types.ActiveBTCDelegation{{}},
				},
			},
			expectErr: "",
		},
		{
			name: "valid SlashedDel event",
			event: types.ConsumerEvent{
				ConsumerId: "consumer1",
				Events: &types.BTCStakingIBCPacket{
					SlashedDel: []*types.SlashedBTCDelegation{{}},
				},
			},
			expectErr: "",
		},
		{
			name: "valid UnbondedDel event",
			event: types.ConsumerEvent{
				ConsumerId: "consumer1",
				Events: &types.BTCStakingIBCPacket{
					UnbondedDel: []*types.UnbondedBTCDelegation{{}},
				},
			},
			expectErr: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.event.Validate()
			if tc.expectErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectErr)
			}
		})
	}
}
