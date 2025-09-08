package mocks

import (
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func CreateValidator(valAddr sdk.ValAddress, stake math.Int) (stakingtypes.Validator, error) {
	pk := ed25519.GenPrivKey().PubKey()
	val, err := stakingtypes.NewValidator(valAddr.String(), pk, stakingtypes.Description{Moniker: "TestValidator"})
	val.Tokens = stake
	val.DelegatorShares = math.LegacyNewDecFromInt(val.Tokens)
	return val, err
}
