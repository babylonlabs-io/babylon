// This file is derived from the Cosmos Relayer repository (https://github.com/cosmos/relayer),
// originally licensed under the Apache License, Version 2.0.

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
