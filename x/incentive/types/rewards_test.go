package types_test

import (
	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/incentive/types"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestEventsPowerUpdateAtHeight_Validate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name    string
		events  *types.EventsPowerUpdateAtHeight
		wantErr string
	}{
		{
			name: "valid - empty events",
			events: &types.EventsPowerUpdateAtHeight{
				Events: nil,
			},
			wantErr: "",
		},
		{
			name: "valid - single BTC activated event",
			events: &types.EventsPowerUpdateAtHeight{
				Events: []*types.EventPowerUpdate{
					{
						Ev: &types.EventPowerUpdate_BtcActivated{
							BtcActivated: &types.EventBTCDelegationActivated{
								FpAddr:     datagen.GenRandomAddress().String(),
								BtcDelAddr: datagen.GenRandomAddress().String(),
								TotalSat:   math.NewInt(1000),
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "valid - single BTC unbonded event",
			events: &types.EventsPowerUpdateAtHeight{
				Events: []*types.EventPowerUpdate{
					{
						Ev: &types.EventPowerUpdate_BtcUnbonded{
							BtcUnbonded: &types.EventBTCDelegationUnbonded{
								FpAddr:     datagen.GenRandomAddress().String(),
								BtcDelAddr: datagen.GenRandomAddress().String(),
								TotalSat:   math.NewInt(1000),
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "valid - multiple mixed events",
			events: &types.EventsPowerUpdateAtHeight{
				Events: []*types.EventPowerUpdate{
					{
						Ev: &types.EventPowerUpdate_BtcActivated{
							BtcActivated: &types.EventBTCDelegationActivated{
								FpAddr:     datagen.GenRandomAddress().String(),
								BtcDelAddr: datagen.GenRandomAddress().String(),
								TotalSat:   math.NewInt(1000),
							},
						},
					},
					{
						Ev: &types.EventPowerUpdate_BtcUnbonded{
							BtcUnbonded: &types.EventBTCDelegationUnbonded{
								FpAddr:     datagen.GenRandomAddress().String(),
								BtcDelAddr: datagen.GenRandomAddress().String(),
								TotalSat:   math.NewInt(2000),
							},
						},
					},
				},
			},
			wantErr: "",
		},
		{
			name: "invalid - empty FP address in activated event",
			events: &types.EventsPowerUpdateAtHeight{
				Events: []*types.EventPowerUpdate{
					{
						Ev: &types.EventPowerUpdate_BtcActivated{
							BtcActivated: &types.EventBTCDelegationActivated{
								FpAddr:     "",
								BtcDelAddr: datagen.GenRandomAddress().String(),
								TotalSat:   math.NewInt(1000),
							},
						},
					},
				},
			},
			wantErr: "empty address",
		},
		{
			name: "invalid - empty delegator address in unbonded event",
			events: &types.EventsPowerUpdateAtHeight{
				Events: []*types.EventPowerUpdate{
					{
						Ev: &types.EventPowerUpdate_BtcUnbonded{
							BtcUnbonded: &types.EventBTCDelegationUnbonded{
								FpAddr:     datagen.GenRandomAddress().String(),
								BtcDelAddr: "",
								TotalSat:   math.NewInt(1000),
							},
						},
					},
				},
			},
			wantErr: "empty address",
		},
		{
			name: "invalid - negative total sat in activated event",
			events: &types.EventsPowerUpdateAtHeight{
				Events: []*types.EventPowerUpdate{
					{
						Ev: &types.EventPowerUpdate_BtcActivated{
							BtcActivated: &types.EventBTCDelegationActivated{
								FpAddr:     datagen.GenRandomAddress().String(),
								BtcDelAddr: datagen.GenRandomAddress().String(),
								TotalSat:   math.NewInt(-1000),
							},
						},
					},
				},
			},
			wantErr: "must be positive",
		},
		{
			name: "invalid - zero total sat in unbonded event",
			events: &types.EventsPowerUpdateAtHeight{
				Events: []*types.EventPowerUpdate{
					{
						Ev: &types.EventPowerUpdate_BtcUnbonded{
							BtcUnbonded: &types.EventBTCDelegationUnbonded{
								FpAddr:     datagen.GenRandomAddress().String(),
								BtcDelAddr: datagen.GenRandomAddress().String(),
								TotalSat:   math.NewInt(0),
							},
						},
					},
				},
			},
			wantErr: "must be positive",
		},
		{
			name: "invalid - nil event in list",
			events: &types.EventsPowerUpdateAtHeight{
				Events: []*types.EventPowerUpdate{
					nil,
				},
			},
			wantErr: "nil event",
		},
		{
			name: "invalid - nil event type",
			events: &types.EventsPowerUpdateAtHeight{
				Events: []*types.EventPowerUpdate{
					{
						Ev: nil,
					},
				},
			},
			wantErr: "nil event type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.events.Validate()
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}
