package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ensure that these message types implement the sdk.Msg interface
var (
	_ sdk.Msg = &MsgWithdrawReward{}
	_ sdk.Msg = &MsgUpdateParams{}
	_ sdk.Msg = &MsgSetWithdrawAddress{}
)
