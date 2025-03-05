package ante

import (
	ckpttypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func InternalInjectedMsg(msgs []sdk.Msg) bool {
	if len(msgs) != 1 {
		return false
	}

	_, ok := msgs[0].(*ckpttypes.MsgInjectedCheckpoint)
	return ok
}
