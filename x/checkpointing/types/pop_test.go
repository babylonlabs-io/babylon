package types_test

import (
	"github.com/babylonlabs-io/babylon/crypto/bls12381"
	"github.com/babylonlabs-io/babylon/privval"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProofOfPossession_IsValid(t *testing.T) {
	valPrivKey := ed25519.GenPrivKey()
	blsPrivKey := bls12381.GenPrivKey()
	pop, err := privval.BuildPoP(valPrivKey, blsPrivKey)
	require.NoError(t, err)
	valpk, err := codec.FromCmtPubKeyInterface(valPrivKey.PubKey())
	require.NoError(t, err)
	require.True(t, pop.IsValid(blsPrivKey.PubKey(), valpk))
}
