package checkpointing_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing"
	"github.com/cometbft/cometbft/crypto/ed25519"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cosmosed "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	simapp "github.com/babylonlabs-io/babylon/v4/app"
	appsigner "github.com/babylonlabs-io/babylon/v4/app/signer"
	"github.com/babylonlabs-io/babylon/v4/x/checkpointing/types"
)

func TestInitGenesis(t *testing.T) {
	app := simapp.Setup(t, false)
	ctx := app.BaseApp.NewContext(false)
	ckptKeeper := app.CheckpointingKeeper

	valNum := 10
	genKeys := make([]*types.GenesisKey, valNum)
	for i := 0; i < valNum; i++ {
		valKeys, err := appsigner.NewValidatorKeys(ed25519.GenPrivKey(), bls12381.GenPrivKey())
		require.NoError(t, err)
		valPubkey, err := cryptocodec.FromCmtPubKeyInterface(valKeys.ValPubkey)
		require.NoError(t, err)
		genKey, err := types.NewGenesisKey(
			sdk.ValAddress(valKeys.ValPubkey.Address()),
			&valKeys.BlsPubkey,
			valKeys.PoP,
			&cosmosed.PubKey{Key: valPubkey.Bytes()},
		)
		require.NoError(t, err)
		genKeys[i] = genKey
	}
	genesisState := types.GenesisState{
		GenesisKeys: genKeys,
	}

	checkpointing.InitGenesis(ctx, ckptKeeper, genesisState)
	for i := 0; i < valNum; i++ {
		addr, err := sdk.ValAddressFromBech32(genKeys[i].ValidatorAddress)
		require.NoError(t, err)
		blsKey, err := ckptKeeper.GetBlsPubKey(ctx, addr)
		require.NoError(t, err)
		require.True(t, genKeys[i].BlsKey.Pubkey.Equal(blsKey))
	}
}
