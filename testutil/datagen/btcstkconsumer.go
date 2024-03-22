package datagen

import (
	"math/rand"

	bsctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
)

func GenRandomChainRegister(r *rand.Rand) *bsctypes.ChainRegister {
	return &bsctypes.ChainRegister{
		ChainId:          "test-" + GenRandomHexStr(r, 10),
		ChainName:        GenRandomHexStr(r, 5),
		ChainDescription: "Chain description: " + GenRandomHexStr(r, 15),
	}
}
