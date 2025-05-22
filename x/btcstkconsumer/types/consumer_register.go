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
	if m.MaxMultiStakedFps == 0 {
		return fmt.Errorf("MaxMultiStakedFps must be greater than 0")
	}
	return nil
}

func NewCosmosConsumerRegister(consumerId, consumerName, consumerDescription string, maxMultiStakedFps uint32) *ConsumerRegister {
	return &ConsumerRegister{
		ConsumerId:          consumerId,
		ConsumerName:        consumerName,
		ConsumerDescription: consumerDescription,
		MaxMultiStakedFps:   maxMultiStakedFps,
		ConsumerMetadata: &ConsumerRegister_CosmosConsumerMetadata{
			CosmosConsumerMetadata: &CosmosConsumerMetadata{},
		},
	}
}

func NewETHL2ConsumerRegister(consumerId, consumerName, consumerDescription string, ethL2FinalityContractAddress string, maxMultiStakedFps uint32) *ConsumerRegister {
	return &ConsumerRegister{
		ConsumerId:          consumerId,
		ConsumerName:        consumerName,
		ConsumerDescription: consumerDescription,
		MaxMultiStakedFps:   maxMultiStakedFps,
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

func (cr *ConsumerRegister) ToResponse() *ConsumerRegisterResponse {
	resp := &ConsumerRegisterResponse{
		ConsumerId:          cr.ConsumerId,
		ConsumerName:        cr.ConsumerName,
		ConsumerDescription: cr.ConsumerDescription,
		MaxMultiStakedFps:   cr.MaxMultiStakedFps,
	}
	if cr.ConsumerMetadata != nil {
		switch md := cr.ConsumerMetadata.(type) {
		case *ConsumerRegister_CosmosConsumerMetadata:
			resp.CosmosChannelId = md.CosmosConsumerMetadata.ChannelId
		case *ConsumerRegister_EthL2ConsumerMetadata:
			resp.EthL2FinalityContractAddress = md.EthL2ConsumerMetadata.FinalityContractAddress
		}
	}
	return resp
}

func (cr ConsumerRegister) Validate() error {
	if len(cr.ConsumerId) == 0 {
		return fmt.Errorf("ConsumerId must be non-empty")
	}
	if len(cr.ConsumerName) == 0 {
		return fmt.Errorf("ConsumerName must be non-empty")
	}
	if len(cr.ConsumerDescription) == 0 {
		return fmt.Errorf("ConsumerDescription must be non-empty")
	}
	if cr.MaxMultiStakedFps == 0 {
		return fmt.Errorf("MaxMultiStakedFps must be greater than 0")
	}
	return nil
}
