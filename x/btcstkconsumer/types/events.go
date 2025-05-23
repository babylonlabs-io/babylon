package types

func NewConsumerRegisteredEvent(consumerId, consumerName, consumerDescription string, consumerType ConsumerType) *EventConsumerRegistered {
	return &EventConsumerRegistered{
		ConsumerId:          consumerId,
		ConsumerName:        consumerName,
		ConsumerDescription: consumerDescription,
		ConsumerType:        consumerType,
	}
}
