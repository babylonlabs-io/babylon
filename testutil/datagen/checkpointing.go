package datagen

import (
	"cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func BuildMsgWrappedCreateValidator(addr sdk.AccAddress) (*types.MsgWrappedCreateValidator, error) {
	bondTokens := sdk.TokensFromConsensusPower(10, sdk.DefaultPowerReduction)
	return BuildMsgWrappedCreateValidatorWithAmount(addr, bondTokens)
}

func BuildMsgWrappedCreateValidatorWithAmount(addr sdk.AccAddress, bondTokens math.Int) (*types.MsgWrappedCreateValidator, error) {
	cmtValPrivkey := ed25519.GenPrivKey()
	bondCoin := sdk.NewCoin(appparams.DefaultBondDenom, bondTokens)
	description := stakingtypes.NewDescription("foo_moniker", "", "", "", "")
	commission := stakingtypes.NewCommissionRates(math.LegacyZeroDec(), math.LegacyZeroDec(), math.LegacyZeroDec())

	pk, err := codec.FromCmtPubKeyInterface(cmtValPrivkey.PubKey())
	if err != nil {
		return nil, err
	}

	createValidatorMsg, err := stakingtypes.NewMsgCreateValidator(
		sdk.ValAddress(addr).String(), pk, bondCoin, description, commission, math.OneInt(),
	)
	if err != nil {
		return nil, err
	}
	blsPrivKey := bls12381.GenPrivKey()
	pop, err := appsigner.BuildPoP(cmtValPrivkey, blsPrivKey)
	if err != nil {
		return nil, err
	}
	blsPubKey := blsPrivKey.PubKey()

	return types.NewMsgWrappedCreateValidator(createValidatorMsg, &blsPubKey, pop)
}
