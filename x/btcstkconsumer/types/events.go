package types

func NewConsumerRegisteredEvent(consumerId, consumerName, consumerDescription string, consumerType ConsumerType,
	rollupFinalityContractAddress string, maxMultiStakedFps uint32) *EventConsumerRegistered {
	return &EventConsumerRegistered{
		ConsumerId:          consumerId,
		ConsumerName:        consumerName,
		ConsumerDescription: consumerDescription,
		ConsumerType:        consumerType,
		MaxMultiStakedFps:   maxMultiStakedFps,
		RollupConsumerMetadata: &RollupConsumerMetadata{
			FinalityContractAddress: rollupFinalityContractAddress,
		},
	}
}
