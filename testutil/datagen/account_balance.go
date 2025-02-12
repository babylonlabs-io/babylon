package datagen

import (
	sdkmath "cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/app/params"
	sec256k1 "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func GenRandomAccount() *authtypes.BaseAccount {
	senderPrivKey := sec256k1.GenPrivKey()
	acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0)
	return acc
}

func GenRandomAccountWithPrefix(prefix string) *authtypes.BaseAccount {
	senderPrivKey := sec256k1.GenPrivKey()
	acc := MustNewBaseAccountWithPrefix(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), 0, 0, prefix)
	return acc
}

// MustNewBaseAccountWithPrefix creates a new BaseAccount object for a given blockchain
// Adapted from github.com/cosmos/cosmos-sdk@v0.50.11/x/auth/types/account.go
func MustNewBaseAccountWithPrefix(address sdk.AccAddress, pubKey cryptotypes.PubKey, accountNumber, sequence uint64, prefix string) *authtypes.BaseAccount {
	blockchainAddress, err := sdk.Bech32ifyAddressBytes(prefix, address.Bytes())
	if err != nil {
		panic(err)
	}

	acc := &authtypes.BaseAccount{
		Address:       blockchainAddress,
		AccountNumber: accountNumber,
		Sequence:      sequence,
	}

	err = acc.SetPubKey(pubKey)
	if err != nil {
		panic(err)
	}

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
