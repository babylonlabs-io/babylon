package types

func NewConsumerRegisteredEvent(consumerId, consumerName, consumerDescription string) *EventConsumerRegistered {
	return &EventConsumerRegistered{
		ConsumerId:          consumerId,
		ConsumerName:        consumerName,
		ConsumerDescription: consumerDescription,
	}
}
