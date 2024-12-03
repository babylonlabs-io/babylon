package types

import (
	"fmt"
)

func (m *MsgRegisterConsumer) ValidateBasic() error {
	if len(m.ConsumerId) == 0 {
		return fmt.Errorf("ConsumerId must be non-empty")
	}
	if len(m.ConsumerName) == 0 {
		return fmt.Errorf("ConsumerName must be non-empty")
	}
	if len(m.ConsumerDescription) == 0 {
		return fmt.Errorf("ConsumerDescription must be non-empty")
	}
	return nil
}

func NewCosmosConsumerRegister(consumerId, consumerName, consumerDescription string) *ConsumerRegister {
	return &ConsumerRegister{
		ConsumerId:          consumerId,
		ConsumerName:        consumerName,
		ConsumerDescription: consumerDescription,
		ConsumerMetadata: &ConsumerRegister_CosmosConsumerMetadata{
			CosmosConsumerMetadata: &CosmosConsumerMetadata{},
		},
	}
}

func NewETHL2ConsumerRegister(consumerId, consumerName, consumerDescription string, ethL2FinalityContractAddress string) *ConsumerRegister {
	return &ConsumerRegister{
		ConsumerId:          consumerId,
		ConsumerName:        consumerName,
		ConsumerDescription: consumerDescription,
		ConsumerMetadata: &ConsumerRegister_EthL2ConsumerMetadata{
			EthL2ConsumerMetadata: &ETHL2ConsumerMetadata{
				FinalityContractAddress: ethL2FinalityContractAddress,
			},
		},
	}
}

func (cr *ConsumerRegister) Type() ConsumerType {
	if _, ok := cr.ConsumerMetadata.(*ConsumerRegister_CosmosConsumerMetadata); ok {
		return ConsumerType_COSMOS
	}
	return ConsumerType_ETH_L2
}
