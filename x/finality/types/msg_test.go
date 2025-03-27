package types_test

import (
	"encoding/json"
	fmt "fmt"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/crypto/eots"
	"github.com/babylonlabs-io/babylon/testutil/datagen"
	"github.com/babylonlabs-io/babylon/x/finality/types"
	"github.com/stretchr/testify/require"
)

func FuzzMsgAddFinalitySig(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)

	prCommitList := []*types.PubRandCommit{}
	msgList := []*types.MsgAddFinalitySig{}

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

		prCommitList = append(prCommitList, prCommit)
		msgList = append(msgList, msg)

		// verify the finality signature message
		err = types.VerifyFinalitySig(msg, prCommit)
		require.NoError(t, err)
	})

	testStrJson, _ := json.Marshal(prCommitList)
	fmt.Println(string(testStrJson))
	fmt.Println("--------------------------------")
	testStrJson, _ = json.Marshal(msgList)
	fmt.Println(string(testStrJson))
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
