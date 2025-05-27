package types_test

import (
	"errors"
	"testing"

	"github.com/babylonlabs-io/babylon/v3/testutil/datagen"
	"github.com/babylonlabs-io/babylon/v3/x/checkpointing/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

var (
	pk1      = ed25519.GenPrivKey().PubKey()
	valAddr1 = sdk.ValAddress(pk1.Address())
)

func TestMsgDecode(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	stakingtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	// build MsgWrappedCreateValidator
	msg, err := datagen.BuildMsgWrappedCreateValidatorWithAmount(
		sdk.AccAddress(valAddr1),
		sdk.TokensFromConsensusPower(10, sdk.DefaultPowerReduction),
	)
	require.NoError(t, err)

	// marshal
	msgBytes, err := cdc.MarshalInterface(msg)
	require.NoError(t, err)

	// unmarshal to sdk.Msg interface
	var msg2 sdk.Msg
	err = cdc.UnmarshalInterface(msgBytes, &msg2)
	require.NoError(t, err)

	// type assertion
	msgWithType, ok := msg2.(*types.MsgWrappedCreateValidator)
	require.True(t, ok)

	// ensure msgWithType.MsgCreateValidator.Pubkey with type Any is unmarshaled successfully
	require.NotNil(t, msgWithType.MsgCreateValidator.Pubkey.GetCachedValue())
}

func TestMsgWrappedCreateValidatorValidateBasic(t *testing.T) {
	t.Parallel()

	valid, err := datagen.BuildMsgWrappedCreateValidator(datagen.GenRandomAddress())
	require.NoError(t, err)

	tcs := []struct {
		title string

		msg    types.MsgWrappedCreateValidator
		expErr error
	}{
		{
			"valid",
			*valid,
			nil,
		},
		{
			"invalid: nil MsgCreateValidator",
			types.MsgWrappedCreateValidator{
				Key:                valid.Key,
				MsgCreateValidator: nil,
			},
			errors.New("MsgCreateValidator is nil"),
		},
		{
			"invalid: nil bls",
			types.MsgWrappedCreateValidator{
				Key:                nil,
				MsgCreateValidator: valid.MsgCreateValidator,
			},
			errors.New("BLS key is nil"),
		},
		{
			"invalid: MsgCreateValidator missing something",
			types.MsgWrappedCreateValidator{
				Key:                valid.Key,
				MsgCreateValidator: &stakingtypes.MsgCreateValidator{},
			},
			errors.New("invalid validator address: empty address string is not allowed: invalid address"),
		},
		{
			"invalid: bls missing pop",
			types.MsgWrappedCreateValidator{
				Key:                &types.BlsKey{},
				MsgCreateValidator: valid.MsgCreateValidator,
			},
			errors.New("BLS Proof of Possession is nil"),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			actErr := tc.msg.ValidateBasic()
			if tc.expErr != nil {
				require.EqualError(t, actErr, tc.expErr.Error())
				return
			}
			require.NoError(t, actErr)
		})
	}
}
