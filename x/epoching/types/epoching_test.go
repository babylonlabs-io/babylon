package types_test

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/epoching/types"

	"github.com/stretchr/testify/require"
)

func FuzzEpoch(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		// generate a random epoch
		epochNumber := uint64(r.Int63()) + 1
		curEpochInterval := r.Uint64()%100 + 2
		firstBlockHeight := r.Uint64() + 1

		e := types.Epoch{
			EpochNumber:          epochNumber,
			CurrentEpochInterval: curEpochInterval,
			FirstBlockHeight:     firstBlockHeight,
		}

		lastBlockHeight := firstBlockHeight + curEpochInterval - 1
		require.Equal(t, lastBlockHeight, e.GetLastBlockHeight())
		secondBlockheight := firstBlockHeight + 1
		require.Equal(t, secondBlockheight, e.GetSecondBlockHeight())
	})
}

func TestValidator_Validate(t *testing.T) {
	validAddr := datagen.GenRandomValidatorAddress()
	testCases := []struct {
		name      string
		validator types.Validator
		valid     bool
		errMsg    string
	}{
		{
			name: "valid validator address",
			validator: types.Validator{
				Addr:  validAddr,
				Power: 100,
			},
			valid: true,
		},
		{
			name: "invalid address (empty)",
			validator: types.Validator{
				Addr:  []byte{},
				Power: 100,
			},
			valid:  false,
			errMsg: "empty address string is not allowed",
		},
		{
			name: "invalid address - wrong byte length",
			validator: types.Validator{
				Addr:  validAddr.Bytes()[1:],
				Power: 100,
			},
			valid:  false,
			errMsg: "address length must be 20 or 32 bytes",
		},
		{
			name: "valid address, zero power",
			validator: types.Validator{
				Addr:  validAddr,
				Power: 0,
			},
			valid: true,
		},
		{
			name: "valid address, negative power", // ?? Should this be allowed ??
			validator: types.Validator{
				Addr:  validAddr,
				Power: -10,
			},
			valid: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.validator.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.errMsg)
		})
	}
}

func TestValidatorLifecycle_Validate(t *testing.T) {
	// Generate a valid validator address
	validValAddr := datagen.GenRandomValidatorAddress().String()
	dummyValState := &types.ValStateUpdate{} // assuming empty struct is acceptable for testing

	testCases := []struct {
		name   string
		input  types.ValidatorLifecycle
		valid  bool
		errMsg string
	}{
		{
			name: "valid validator address and lifecycle",
			input: types.ValidatorLifecycle{
				ValAddr: validValAddr,
				ValLife: []*types.ValStateUpdate{dummyValState},
			},
			valid: true,
		},
		{
			name: "empty validator lifecycle",
			input: types.ValidatorLifecycle{
				ValAddr: validValAddr,
				ValLife: []*types.ValStateUpdate{},
			},
			valid:  false,
			errMsg: "validator lyfecycle is empty",
		},
		{
			name: "nil validator lifecycle",
			input: types.ValidatorLifecycle{
				ValAddr: validValAddr,
				ValLife: nil,
			},
			valid:  false,
			errMsg: "validator lyfecycle is empty",
		},
		{
			name: "invalid validator address",
			input: types.ValidatorLifecycle{
				ValAddr: "invalid-bech32",
				ValLife: []*types.ValStateUpdate{dummyValState},
			},
			valid:  false,
			errMsg: "decoding bech32 failed",
		},
		{
			name: "empty address and empty lifecycle",
			input: types.ValidatorLifecycle{
				ValAddr: "",
				ValLife: []*types.ValStateUpdate{},
			},
			valid:  false,
			errMsg: "validator lyfecycle is empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.errMsg)
		})
	}
}

func TestDelegationLifecycle_Validate(t *testing.T) {
	// Generate a valid delegator address
	validDelAddr := datagen.GenRandomAddress().String()
	dummyDelState := &types.DelegationStateUpdate{} // Assuming an empty update is valid for testing

	testCases := []struct {
		name   string
		input  types.DelegationLifecycle
		valid  bool
		errMsg string
	}{
		{
			name: "valid delegation lifecycle",
			input: types.DelegationLifecycle{
				DelAddr: validDelAddr,
				DelLife: []*types.DelegationStateUpdate{dummyDelState},
			},
			valid: true,
		},
		{
			name: "empty DelLife",
			input: types.DelegationLifecycle{
				DelAddr: validDelAddr,
				DelLife: []*types.DelegationStateUpdate{},
			},
			valid:  false,
			errMsg: "delegation lyfecycle is empty",
		},
		{
			name: "nil DelLife",
			input: types.DelegationLifecycle{
				DelAddr: validDelAddr,
				DelLife: nil,
			},
			valid:  false,
			errMsg: "delegation lyfecycle is empty",
		},
		{
			name: "invalid DelAddr",
			input: types.DelegationLifecycle{
				DelAddr: "not-a-valid-bech32",
				DelLife: []*types.DelegationStateUpdate{dummyDelState},
			},
			valid:  false,
			errMsg: "decoding bech32 failed",
		},
		{
			name: "invalid DelAddr - empty addr",
			input: types.DelegationLifecycle{
				DelAddr: "",
				DelLife: []*types.DelegationStateUpdate{dummyDelState},
			},
			valid:  false,
			errMsg: "empty address string is not allowed",
		},
		{
			name: "invalid DelAddr and empty DelLife",
			input: types.DelegationLifecycle{
				DelAddr: "",
				DelLife: []*types.DelegationStateUpdate{},
			},
			valid:  false,
			errMsg: "delegation lyfecycle is empty",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tc.errMsg)
		})
	}
}
