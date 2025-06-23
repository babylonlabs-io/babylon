package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCreateFinalityProvider{}, "btcstaking/MsgCreateFinalityProvider", nil)
	cdc.RegisterConcrete(&MsgEditFinalityProvider{}, "btcstaking/MsgEditFinalityProvider", nil)
	cdc.RegisterConcrete(&MsgCreateBTCDelegation{}, "btcstaking/MsgCreateBTCDelegation", nil)
	cdc.RegisterConcrete(&MsgAddCovenantSigs{}, "btcstaking/MsgAddCovenantSigs", nil)
	cdc.RegisterConcrete(&MsgBTCUndelegate{}, "btcstaking/MsgBTCUndelegate", nil)
	cdc.RegisterConcrete(&MsgUpdateParams{}, "btcstaking/MsgUpdateParams", nil)
	cdc.RegisterConcrete(&MsgAddBTCDelegationInclusionProof{}, "btcstaking/MsgAddBTCDelegationInclusionProof", nil)
	cdc.RegisterConcrete(&MsgBtcStakeExpand{}, "btcstaking/MsgBtcStakeExpand", nil)
}

func RegisterInterfaces(registry cdctypes.InterfaceRegistry) {
	// Register messages
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgCreateFinalityProvider{},
		&MsgEditFinalityProvider{},
		&MsgCreateBTCDelegation{},
		&MsgAddCovenantSigs{},
		&MsgBTCUndelegate{},
		&MsgUpdateParams{},
		&MsgAddBTCDelegationInclusionProof{},
		&MsgBtcStakeExpand{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

var (
	Amino     = codec.NewLegacyAmino()
	ModuleCdc = codec.NewProtoCodec(cdctypes.NewInterfaceRegistry())
)
