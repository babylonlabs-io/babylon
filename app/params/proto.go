//go:build !test_amino
// +build !test_amino

package params

import (
	"cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	gogoproto "github.com/cosmos/gogoproto/proto"
	enccodec "github.com/evmos/ethermint/encoding/codec"
)

// DefaultEncodingConfig returns the default encoding config
func DefaultEncodingConfig() *EncodingConfig {
	cdc := codec.NewLegacyAmino()
	signingOptions := signing.Options{
		AddressCodec: address.Bech32Codec{
			Bech32Prefix: sdk.GetConfig().GetBech32AccountAddrPrefix(),
		},
		ValidatorAddressCodec: address.Bech32Codec{
			Bech32Prefix: sdk.GetConfig().GetBech32ValidatorAddrPrefix(),
		},
	}

	interfaceRegistry, err := types.NewInterfaceRegistryWithOptions(types.InterfaceRegistryOptions{
		ProtoFiles:     gogoproto.HybridResolver,
		SigningOptions: signingOptions,
	})
	if err != nil {
		panic(err)
	}
	if err := interfaceRegistry.SigningContext().Validate(); err != nil {
		panic(err)
	}

	codec := codec.NewProtoCodec(interfaceRegistry)

	encodingConfig := &EncodingConfig{
		InterfaceRegistry: interfaceRegistry,
		Codec:             codec,
		TxConfig:          tx.NewTxConfig(codec, tx.DefaultSignModes),
		Amino:             cdc,
	}
	enccodec.RegisterLegacyAminoCodec(cdc)
	enccodec.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	return encodingConfig
}
