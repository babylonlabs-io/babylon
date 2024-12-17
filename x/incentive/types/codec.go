package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgWithdrawReward{}, "incentive/MsgWithdrawReward", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "incentive/MsgUpdateParams", nil)
	cdc.RegisterConcrete(&MsgSetWithdrawAddress{}, "incentive/MsgSetWithdrawAddress", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	// Register messages
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgWithdrawReward{},
		&MsgUpdateParams{},
		&MsgSetWithdrawAddress{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
