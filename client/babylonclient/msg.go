// This file is derived from the Cosmos Relayer repository (https://github.com/cosmos/relayer),
// originally licensed under the Apache License, Version 2.0.

package babylonclient

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
)

type CosmosMessage struct {
	Msg       sdk.Msg
	SetSigner func(string) // callback to update the Msg Signer field
}

func NewCosmosMessage(msg sdk.Msg, optionalSetSigner func(string)) RelayerMessage {
	return CosmosMessage{
		Msg:       msg,
		SetSigner: optionalSetSigner,
	}
}

func CosmosMsgs(rm ...RelayerMessage) []sdk.Msg {
	sdkMsgs := make([]sdk.Msg, 0)
	for _, rMsg := range rm {
		if val, ok := rMsg.(CosmosMessage); !ok {
			fmt.Printf("got data of type %T but wanted CosmosMessage \n", rMsg)
			return nil
		} else {
			sdkMsgs = append(sdkMsgs, val.Msg)
		}
	}
	return sdkMsgs
}

func (cm CosmosMessage) Type() string {
	return sdk.MsgTypeURL(cm.Msg)
}

func (cm CosmosMessage) MsgBytes() ([]byte, error) {
	return proto.Marshal(cm.Msg)
}
