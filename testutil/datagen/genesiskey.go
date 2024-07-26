package datagen

import (
	"github.com/babylonlabs-io/babylon/app"
	appkeepers "github.com/babylonlabs-io/babylon/app/keepers"
	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/privval"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
	cmtcrypto "github.com/cometbft/cometbft/crypto"
	cmted25519 "github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/codec"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cosmosed "github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type GenesisValidators struct {
	Keys []*GenesisKeyWithBLS
}

type GenesisKeyWithBLS struct {
	checkpointingtypes.GenesisKey
	bls12381.PrivateKey
	cmtcrypto.PrivKey
}

func (gvs *GenesisValidators) GetGenesisKeys() []*checkpointingtypes.GenesisKey {
	gensisKeys := make([]*checkpointingtypes.GenesisKey, 0, len(gvs.Keys))
	for _, k := range gvs.Keys {
		gensisKeys = append(gensisKeys, &k.GenesisKey)
	}

	return gensisKeys
}

func (gvs *GenesisValidators) GetBLSPrivKeys() []bls12381.PrivateKey {
	blsPrivKeys := make([]bls12381.PrivateKey, 0, len(gvs.Keys))
	for _, k := range gvs.Keys {
		blsPrivKeys = append(blsPrivKeys, k.PrivateKey)
	}

	return blsPrivKeys
}

func (gvs *GenesisValidators) GetValPrivKeys() []cmtcrypto.PrivKey {
	valPrivKeys := make([]cmtcrypto.PrivKey, 0, len(gvs.Keys))
	for _, k := range gvs.Keys {
		valPrivKeys = append(valPrivKeys, k.PrivKey)
	}

	return valPrivKeys
}

// GenesisValidatorSet generates a set with `numVals` genesis validators
func GenesisValidatorSet(numVals int) (*GenesisValidators, error) {
	genesisVals := make([]*GenesisKeyWithBLS, 0, numVals)
	for i := 0; i < numVals; i++ {
		blsPrivKey := bls12381.GenPrivKey()
		// create validator set with single validator
		valPrivKey := cmted25519.GenPrivKey()
		valKeys, err := privval.NewValidatorKeys(valPrivKey, blsPrivKey)
		if err != nil {
			return nil, err
		}
		valPubkey, err := cryptocodec.FromCmtPubKeyInterface(valKeys.ValPubkey)
		if err != nil {
			return nil, err
		}
		genesisKey, err := checkpointingtypes.NewGenesisKey(
			sdk.ValAddress(valKeys.ValPubkey.Address()),
			&valKeys.BlsPubkey,
			valKeys.PoP,
			&cosmosed.PubKey{Key: valPubkey.Bytes()},
		)
		if err != nil {
			return nil, err
		}
		genesisVals = append(genesisVals, &GenesisKeyWithBLS{
			GenesisKey: *genesisKey,
			PrivateKey: blsPrivKey,
			PrivKey:    valPrivKey,
		})
	}

	return &GenesisValidators{Keys: genesisVals}, nil
}

// GenesisValidatorSetWithPrivSigner generates a set with `numVals` genesis validators
// along with the privSigner, which will be in the 0th position of the return validator set
func GenesisValidatorSetWithPrivSigner(numVals int) (*GenesisValidators, *appkeepers.PrivSigner, error) {
	ps, err := app.SetupTestPrivSigner()
	if err != nil {
		return nil, nil, err
	}
	signerGenesisKey, err := app.GenesisKeyFromPrivSigner(ps)
	if err != nil {
		return nil, nil, err
	}
	signerVal := &GenesisKeyWithBLS{
		GenesisKey: *signerGenesisKey,
		PrivateKey: ps.WrappedPV.Key.BlsPrivKey,
		PrivKey:    ps.WrappedPV.Key.PrivKey,
	}
	genesisVals, err := GenesisValidatorSet(numVals)
	if err != nil {
		return nil, nil, err
	}
	genesisVals.Keys[0] = signerVal

	return genesisVals, ps, nil
}

func GenerateGenesisKey() *checkpointingtypes.GenesisKey {
	accPrivKey := secp256k1.GenPrivKey()
	tmValPrivKey := cmted25519.GenPrivKey()
	blsPrivKey := bls12381.GenPrivKey()
	tmValPubKey := tmValPrivKey.PubKey()
	valPubKey, err := codec.FromCmtPubKeyInterface(tmValPubKey)
	if err != nil {
		panic(err)
	}

	blsPubKey := blsPrivKey.PubKey()
	address := sdk.ValAddress(accPrivKey.PubKey().Address())
	pop, err := privval.BuildPoP(tmValPrivKey, blsPrivKey)
	if err != nil {
		panic(err)
	}

	gk, err := checkpointingtypes.NewGenesisKey(address, &blsPubKey, pop, valPubKey)
	if err != nil {
		panic(err)
	}

	return gk
}
