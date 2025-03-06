package ante

import (
	ckpttypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func SingleInjectedMsg(msgs []sdk.Msg) bool {
	return len(msgs) == 1 && InjectedMsg(msgs[0])
}

func InjectedMsg(msg sdk.Msg) bool {
	_, ok := msg.(*ckpttypes.MsgInjectedCheckpoint)
	return ok
}
