package types

import (
	"context"

	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
)

type ClientKeeper interface {
	GetClientState(ctx sdk.Context, clientID string) (ibcexported.ClientState, bool)
}

type ChannelKeeper interface {
	GetChannelClientState(ctx sdk.Context, portID, channelID string) (clientID string, state ibcexported.ClientState, err error)
	ConsumerHasIBCChannelOpen(ctx context.Context, consumerID, channelID string) bool
}

type WasmKeeper interface {
	GetContractInfo(ctx context.Context, contractAddress sdk.AccAddress) *wasmtypes.ContractInfo
}
