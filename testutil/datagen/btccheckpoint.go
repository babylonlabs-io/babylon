package datagen

import (
	"math/rand"

	"github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
)

func RandomTxKey(r *rand.Rand) *types.TransactionKey {
	header := GenRandomBTCHeaderBytes(r, nil, nil)
	return &types.TransactionKey{Index: RandomUInt32(r, 10000), Hash: header.Hash()}
}

func RandomBtcStatus(r *rand.Rand) types.BtcStatus {
	return types.BtcStatus(rand.Intn(3))
}
