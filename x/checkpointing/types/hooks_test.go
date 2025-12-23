package types_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
)

type mockCheckpointingHook struct {
	blsKeyRegisteredCalls            int
	rawCheckpointSealedCalls         int
	rawCheckpointConfirmedCalls      int
	rawCheckpointForgottenCalls      int
	rawCheckpointFinalizedCalls      int
	rawCheckpointBlsSigVerifiedCalls int
	shouldReturnError                bool
}

func (m *mockCheckpointingHook) AfterBlsKeyRegistered(ctx context.Context, valAddr sdk.ValAddress) error {
	m.blsKeyRegisteredCalls++
	if m.shouldReturnError {
		return errors.New("mock error")
	}
	return nil
}

func (m *mockCheckpointingHook) AfterRawCheckpointSealed(ctx context.Context, epoch uint64) error {
	m.rawCheckpointSealedCalls++
	if m.shouldReturnError {
		return errors.New("mock error")
	}
	return nil
}

func (m *mockCheckpointingHook) AfterRawCheckpointConfirmed(ctx context.Context, epoch uint64) error {
	m.rawCheckpointConfirmedCalls++
	if m.shouldReturnError {
		return errors.New("mock error")
	}
	return nil
}

func (m *mockCheckpointingHook) AfterRawCheckpointForgotten(ctx context.Context, ckpt *types.RawCheckpoint) error {
	m.rawCheckpointForgottenCalls++
	if m.shouldReturnError {
		return errors.New("mock error")
	}
	return nil
}

func (m *mockCheckpointingHook) AfterRawCheckpointFinalized(ctx context.Context, epoch uint64) error {
	m.rawCheckpointFinalizedCalls++
	if m.shouldReturnError {
		return errors.New("mock error")
	}
	return nil
}

func (m *mockCheckpointingHook) AfterRawCheckpointBlsSigVerified(ctx context.Context, ckpt *types.RawCheckpoint) error {
	m.rawCheckpointBlsSigVerifiedCalls++
	if m.shouldReturnError {
		return errors.New("mock error")
	}
	return nil
}

func TestMultiCheckpointingHooks_AfterRawCheckpointForgotten(t *testing.T) {
	tests := []struct {
		name             string
		hook1ShouldError bool
		hook2ShouldError bool
		hook3ShouldError bool
		expErr           error
		expHook1Calls    int
		expHook2Calls    int
		expHook3Calls    int
	}{
		{
			name:             "all hooks called successfully",
			hook1ShouldError: false,
			hook2ShouldError: false,
			hook3ShouldError: false,
			expErr:           nil,
			expHook1Calls:    1,
			expHook2Calls:    1,
			expHook3Calls:    1,
		},
		{
			name:             "stops on second hook error",
			hook1ShouldError: false,
			hook2ShouldError: true,
			hook3ShouldError: false,
			expErr:           errors.New("mock error"),
			expHook1Calls:    1,
			expHook2Calls:    1,
			expHook3Calls:    0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			r := rand.New(rand.NewSource(time.Now().UnixNano()))

			hook1 := &mockCheckpointingHook{shouldReturnError: tc.hook1ShouldError}
			hook2 := &mockCheckpointingHook{shouldReturnError: tc.hook2ShouldError}
			hook3 := &mockCheckpointingHook{shouldReturnError: tc.hook3ShouldError}

			multiHooks := types.NewMultiCheckpointingHooks(hook1, hook2, hook3)

			ckpt := datagen.GenRandomRawCheckpoint(r)

			actErr := multiHooks.AfterRawCheckpointForgotten(ctx, ckpt)

			if tc.expErr != nil {
				require.EqualError(t, actErr, tc.expErr.Error())
			} else {
				require.NoError(t, actErr)
			}

			require.Equal(t, tc.expHook1Calls, hook1.rawCheckpointForgottenCalls, "hook1 call count mismatch")
			require.Equal(t, tc.expHook2Calls, hook2.rawCheckpointForgottenCalls, "hook2 call count mismatch")
			require.Equal(t, tc.expHook3Calls, hook3.rawCheckpointForgottenCalls, "hook3 call count mismatch")
		})
	}
}

func TestMultiCheckpointingHooks_AfterRawCheckpointForgotten_AllHooksCalled(t *testing.T) {
	hook1 := &mockCheckpointingHook{}
	hook2 := &mockCheckpointingHook{}
	hook3 := &mockCheckpointingHook{}

	multiHooks := types.NewMultiCheckpointingHooks(hook1, hook2, hook3)

	ckpt := &types.RawCheckpoint{EpochNum: 1}
	err := multiHooks.AfterRawCheckpointForgotten(context.Background(), ckpt)

	require.NoError(t, err)
	require.Equal(t, 1, hook1.rawCheckpointForgottenCalls, "first hook should be called")
	require.Equal(t, 1, hook2.rawCheckpointForgottenCalls, "second hook should be called")
	require.Equal(t, 1, hook3.rawCheckpointForgottenCalls, "third hook should be called")
}

func TestMultiCheckpointingHooks_OtherMethods_AllHooksCalled(t *testing.T) {
	hook1 := &mockCheckpointingHook{}
	hook2 := &mockCheckpointingHook{}

	multiHooks := types.NewMultiCheckpointingHooks(hook1, hook2)
	ctx := context.Background()

	err := multiHooks.AfterRawCheckpointSealed(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, hook1.rawCheckpointSealedCalls)
	require.Equal(t, 1, hook2.rawCheckpointSealedCalls)

	err = multiHooks.AfterRawCheckpointConfirmed(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, hook1.rawCheckpointConfirmedCalls)
	require.Equal(t, 1, hook2.rawCheckpointConfirmedCalls)

	err = multiHooks.AfterRawCheckpointFinalized(ctx, 1)
	require.NoError(t, err)
	require.Equal(t, 1, hook1.rawCheckpointFinalizedCalls)
	require.Equal(t, 1, hook2.rawCheckpointFinalizedCalls)

	ckpt := &types.RawCheckpoint{EpochNum: 1}
	err = multiHooks.AfterRawCheckpointBlsSigVerified(ctx, ckpt)
	require.NoError(t, err)
	require.Equal(t, 1, hook1.rawCheckpointBlsSigVerifiedCalls)
	require.Equal(t, 1, hook2.rawCheckpointBlsSigVerifiedCalls)
}
