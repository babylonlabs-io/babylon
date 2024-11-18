package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCommitPubRandList{}, "finality/MsgCommitPubRandList", nil)
	cdc.RegisterConcrete(&MsgAddFinalitySig{}, "finality/MsgAddFinalitySig", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "finality/MsgUpdateParams", nil)
	cdc.RegisterConcrete(&MsgResumeFinalityProposal{}, "finality/MsgResumeFinalityProposal", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	// Register messages
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgCommitPubRandList{},
		&MsgAddFinalitySig{},
		&MsgUpdateParams{},
		&MsgResumeFinalityProposal{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
