package datagen

import (
	"errors"
	"strings"

	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v3/app/params"
	sec256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func GenRandomSecp256k1Address() sdk.AccAddress {
	senderPrivKey := sec256k1.GenPrivKey()
	return senderPrivKey.PubKey().Address().Bytes()
}

func GenRandomAccount() *authtypes.BaseAccount {
	senderPrivKey := sec256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	return acc
}

func GenRandomAccWithBalance(n int) ([]authtypes.GenesisAccount, []banktypes.Balance) {
	accs := make([]authtypes.GenesisAccount, n)
	balances := make([]banktypes.Balance, n)
	for i := 0; i < n; i++ {
		senderPrivKey := sec256k1.GenPrivKey()
		acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
		accs[i] = acc
		balance := banktypes.Balance{
			Address: acc.GetAddress().String(),
			Coins:   sdk.NewCoins(sdk.NewCoin(appparams.DefaultBondDenom, sdkmath.NewInt(100000000000000))),
		}
		balances[i] = balance
	}

	return accs, balances
}

// MustAccAddressFromBech32WithPrefix calls AccAddressFromBech32WithPrefix and
// panics on error.
// Adapted from github.com/cosmos/cosmos-sdk@v0.50.11/types/address.go
func MustAccAddressFromBech32WithPrefix(address, prefix string) sdk.AccAddress {
	addr, err := AccAddressFromBech32WithPrefix(address, prefix)
	if err != nil {
		panic(err)
	}

	return addr
}

// AccAddressFromBech32WithPrefix creates an AccAddress from a Bech32 string.
// Adapted from github.com/cosmos/cosmos-sdk@v0.50.11/types/address.go
func AccAddressFromBech32WithPrefix(address, bech32PrefixAccAddr string) (addr sdk.AccAddress, err error) {
	if len(strings.TrimSpace(address)) == 0 {
		return sdk.AccAddress{}, errors.New("empty address string is not allowed")
	}

	bz, err := sdk.GetFromBech32(address, bech32PrefixAccAddr)
	if err != nil {
		return nil, err
	}

	err = sdk.VerifyAddressFormat(bz)
	if err != nil {
		return nil, err
	}

	return bz, nil
}
