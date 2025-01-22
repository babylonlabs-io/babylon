// This file is derived from the Cosmos Relayer repository (https://github.com/cosmos/relayer),
// originally licensed under the Apache License, Version 2.0.

package babylonclient

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetKeyAddress returns the account address representation for the currently configured key.
func (cc *CosmosProvider) GetKeyAddress(key string) (sdk.AccAddress, error) {
	info, err := cc.Keybase.Key(key)
	if err != nil {
		return nil, err
	}
	return info.GetAddress()
}

// EncodeBech32AccAddr returns the string bech32 representation for the specified account address.
// It returns an empty sting if the byte slice is 0-length.
// It returns an error if the bech32 conversion fails or the prefix is empty.
func (cc *CosmosProvider) EncodeBech32AccAddr(addr sdk.AccAddress) (string, error) {
	return sdk.Bech32ifyAddressBytes(cc.PCfg.AccountPrefix, addr)
}

func (cc *CosmosProvider) DecodeBech32AccAddr(addr string) (sdk.AccAddress, error) {
	return sdk.GetFromBech32(addr, cc.PCfg.AccountPrefix)
}

func (cc *CosmosProvider) GetKeyAddressForKey(key string) (sdk.AccAddress, error) {
	info, err := cc.Keybase.Key(key)
	if err != nil {
		return nil, err
	}
	return info.GetAddress()
}
