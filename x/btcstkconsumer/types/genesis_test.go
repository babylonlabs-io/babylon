package types_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/btcstkconsumer/types"

	"github.com/stretchr/testify/require"
)

func TestGenesisState_Validate(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))
	entriesCount := rand.Intn(25) + 2 // make sure it is always at least 2
	consumers := make([]*types.ConsumerRegister, 0, entriesCount)

	for range entriesCount {
		consumer := datagen.GenRandomCosmosConsumerRegister(r)
		consumers = append(consumers, consumer)
	}

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
			desc:     "valid genesis state - empty",
			genState: &types.GenesisState{},
			valid:    true,
		},
		{
			desc: "valid genesis state",
			genState: &types.GenesisState{
				Params:    types.DefaultParams(),
				Consumers: consumers,
			},
			valid: true,
		},
		{
			desc: "duplicate consumer ids",
			genState: &types.GenesisState{
				Consumers: []*types.ConsumerRegister{
					consumers[0], consumers[0],
				},
			},
			valid:  false,
			errMsg: "duplicate consumer id",
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
