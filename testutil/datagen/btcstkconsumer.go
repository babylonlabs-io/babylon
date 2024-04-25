package datagen

import (
	"math/rand"

	bsctypes "github.com/babylonchain/babylon/x/btcstkconsumer/types"
)

func GenRandomConsumerRegister(r *rand.Rand) *bsctypes.ConsumerRegister {
	return &bsctypes.ConsumerRegister{
		ConsumerId:          "test-" + GenRandomHexStr(r, 10),
		ConsumerName:        GenRandomHexStr(r, 5),
		ConsumerDescription: "Chain description: " + GenRandomHexStr(r, 15),
	}
}
