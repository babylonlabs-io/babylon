package types

func NewConsumerRegisteredEvent(consumerId, consumerName, consumerDescription string, consumerType ConsumerType,
	rollupFinalityContractAddress string, consumerMaxMultiStakedFps uint32) *EventConsumerRegistered {
	event := &EventConsumerRegistered{
		ConsumerId:                consumerId,
		ConsumerName:              consumerName,
		ConsumerDescription:       consumerDescription,
		ConsumerMaxMultiStakedFps: consumerMaxMultiStakedFps,
		ConsumerType:              consumerType,
	}

	// Only set rollup metadata if it's a rollup consumer
	if consumerType == ConsumerType_ROLLUP && rollupFinalityContractAddress != "" {
		event.RollupConsumerMetadata = &RollupConsumerMetadata{
			FinalityContractAddress: rollupFinalityContractAddress,
		}
	}

	return event
}
