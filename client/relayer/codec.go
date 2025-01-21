package relayerclient

import (
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
)

type Codec struct {
	InterfaceRegistry types.InterfaceRegistry
	Marshaller        codec.Codec
	TxConfig          client.TxConfig
	Amino             *codec.LegacyAmino
}
