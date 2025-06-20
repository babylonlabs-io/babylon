package datagen

import (
	"math/rand"

	bsctypes "github.com/babylonlabs-io/babylon/v3/x/btcstkconsumer/types"
)

func GenRandomCosmosConsumerRegister(r *rand.Rand) *bsctypes.ConsumerRegister {
	clientID := "test-" + GenRandomHexStr(r, 10)
	return &bsctypes.ConsumerRegister{
		ConsumerId:          clientID,
		ConsumerName:        GenRandomHexStr(r, 5),
		ConsumerDescription: "Chain description: " + GenRandomHexStr(r, 15),
		ConsumerMetadata: &bsctypes.ConsumerRegister_CosmosConsumerMetadata{
			CosmosConsumerMetadata: &bsctypes.CosmosConsumerMetadata{},
		},
	}
}

func GenRandomRollupRegister(r *rand.Rand, contractAddress string) *bsctypes.ConsumerRegister {
	clientID := "test-" + GenRandomHexStr(r, 10)
	return &bsctypes.ConsumerRegister{
		ConsumerId:          clientID,
		ConsumerName:        GenRandomHexStr(r, 5),
		ConsumerDescription: "Chain description: " + GenRandomHexStr(r, 15),
		ConsumerMetadata: &bsctypes.ConsumerRegister_RollupConsumerMetadata{
			RollupConsumerMetadata: &bsctypes.RollupConsumerMetadata{
				FinalityContractAddress: contractAddress,
			},
		},
	}
}
