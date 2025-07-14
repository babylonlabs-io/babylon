package datagen

import (
	"math/rand"

	sdkmath "cosmossdk.io/math"
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
		BabylonRewardsCommission: GenBabylonRewardsCommission(r),
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
		BabylonRewardsCommission: GenBabylonRewardsCommission(r),
	}
}

func GenBabylonRewardsCommission(r *rand.Rand) sdkmath.LegacyDec {
	return sdkmath.LegacyNewDecWithPrec(int64(RandomInt(r, 9)+1), 2)
}
