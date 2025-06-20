package types

func NewConsumerRegisteredEvent(consumerId, consumerName, consumerDescription string, consumerType ConsumerType,
	rollupFinalityContractAddress string) *EventConsumerRegistered {
	return &EventConsumerRegistered{
		ConsumerId:          consumerId,
		ConsumerName:        consumerName,
		ConsumerDescription: consumerDescription,
		ConsumerType:        consumerType,
		RollupConsumerMetadata: &RollupConsumerMetadata{
			FinalityContractAddress: rollupFinalityContractAddress,
		},
	}
}
