package types_test

import (
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/crypto/eots"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/x/finality/types"
	"github.com/stretchr/testify/require"
)

func FuzzMsgAddFinalitySig(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		sk, err := eots.KeyGen(r)
		require.NoError(t, err)

		numPubRand := uint64(100)
		randListInfo, err := datagen.GenRandomPubRandList(r, numPubRand)
		require.NoError(t, err)

		startHeight := datagen.RandomInt(r, 10)
		blockHeight := startHeight + datagen.RandomInt(r, 10)
		blockHash := datagen.GenRandomByteArray(r, 32)

		signer := datagen.GenRandomAccount().Address
		msg, err := datagen.NewMsgAddFinalitySig(signer, sk, startHeight, blockHeight, randListInfo, blockHash)
		require.NoError(t, err)

		prCommit := &types.PubRandCommit{
			StartHeight: startHeight,
			NumPubRand:  numPubRand,
			Commitment:  randListInfo.Commitment,
		}

		// verify the finality signature message
		err = types.VerifyFinalitySig(msg, prCommit)
		require.NoError(t, err)
	})
}

func FuzzMsgCommitPubRandList(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))

		sk, _, err := datagen.GenRandomBTCKeyPair(r)
		require.NoError(t, err)

		startHeight := datagen.RandomInt(r, 10)
		numPubRand := datagen.RandomInt(r, 100) + 1
		_, msg, err := datagen.GenRandomMsgCommitPubRandList(r, sk, startHeight, numPubRand)
		require.NoError(t, err)

		// sanity checks, including verifying signature
		err = msg.VerifySig()
		require.NoError(t, err)
	})
}

func TestMsgCommitPubRandListValidateBasic(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	sk, _, err := datagen.GenRandomBTCKeyPair(r)
	require.NoError(t, err)

	tests := []struct {
		name        string
		msgModifier func(*types.MsgCommitPubRandList)
		expectErr   bool
		errString   string
	}{
		{
			name:        "valid message",
			msgModifier: func(msg *types.MsgCommitPubRandList) {},
			expectErr:   false,
		},
		{
			name: "invalid signer",
			msgModifier: func(msg *types.MsgCommitPubRandList) {
				msg.Signer = "invalid-address"
			},
			expectErr: true,
			errString: "invalid signer address",
		},
		{
			name: "invalid commitment size",
			msgModifier: func(msg *types.MsgCommitPubRandList) {
				msg.Commitment = []byte("too-short")
			},
			expectErr: true,
			errString: "commitment must be 32 bytes",
		},
		{
			name: "overflow in block height",
			msgModifier: func(msg *types.MsgCommitPubRandList) {
				msg.NumPubRand = 0
			},
			expectErr: true,
			errString: "public rand commit start block height",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			startHeight := datagen.RandomInt(r, 10)
			numPubRand := datagen.RandomInt(r, 100) + 1
			_, msg, err := datagen.GenRandomMsgCommitPubRandList(r, sk, startHeight, numPubRand)
			require.NoError(t, err)

			tc.msgModifier(msg)

			err = msg.ValidateBasic()

			if tc.expectErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.errString)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
