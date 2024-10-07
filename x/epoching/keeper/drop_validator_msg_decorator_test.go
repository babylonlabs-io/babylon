package keeper

import (
	"testing"

	"github.com/babylonlabs-io/babylon/x/epoching/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authz "github.com/cosmos/cosmos-sdk/x/authz"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func newMsgExecWithStakingMsg() *authz.MsgExec {
	msg := authz.NewMsgExec(sdk.AccAddress("test"), []sdk.Msg{
		&stakingtypes.MsgCreateValidator{},
		&stakingtypes.MsgEditValidator{},
	})
	return &msg
}

func newValidMsgExec() *authz.MsgExec {
	msg := authz.NewMsgExec(sdk.AccAddress("test"), []sdk.Msg{
		&stakingtypes.MsgEditValidator{},
	})
	return &msg
}

func TestDropValidatorMsgDecorator(t *testing.T) {
	testCases := []struct {
		msg       sdk.Msg
		expectErr error
	}{
		// wrapped message types that should be rejected
		{&stakingtypes.MsgCreateValidator{}, types.ErrUnwrappedMsgType},
		{&stakingtypes.MsgDelegate{}, types.ErrUnwrappedMsgType},
		{&stakingtypes.MsgUndelegate{}, types.ErrUnwrappedMsgType},
		{&stakingtypes.MsgBeginRedelegate{}, types.ErrUnwrappedMsgType},
		{&stakingtypes.MsgCancelUnbondingDelegation{}, types.ErrUnwrappedMsgType},
		// MsgExec that contains staking messages should be rejected
		{newMsgExecWithStakingMsg(), types.ErrUnwrappedMsgType},
		// allowed message types
		{&stakingtypes.MsgEditValidator{}, nil},
		{newValidMsgExec(), nil},
	}

	decorator := NewDropValidatorMsgDecorator(&Keeper{})

	for _, tc := range testCases {
		err := decorator.ValidateMsg(tc.msg)
		if tc.expectErr != nil {
			require.Error(t, err)
			require.Equal(t, tc.expectErr, err)
		} else {
			require.NoError(t, err)
		}
	}
}
