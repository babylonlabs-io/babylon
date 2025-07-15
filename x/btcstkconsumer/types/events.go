package types

import (
	"cosmossdk.io/math"
)

func NewConsumerRegisteredEvent(
	consumerId, consumerName, consumerDescription string,
	consumerType ConsumerType,
	rollupFinalityContractAddress string,
	babylonRewardsCommission math.LegacyDec,
) *EventConsumerRegistered {
	return &EventConsumerRegistered{
		ConsumerId:          consumerId,
		ConsumerName:        consumerName,
		ConsumerDescription: consumerDescription,
		ConsumerType:        consumerType,
		RollupConsumerMetadata: &RollupConsumerMetadata{
			FinalityContractAddress: rollupFinalityContractAddress,
		},
		BabylonRewardsCommission: babylonRewardsCommission,
	}
}
