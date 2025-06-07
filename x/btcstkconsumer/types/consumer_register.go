package types

func (m *MsgRegisterConsumer) ValidateBasic() error {
	if len(m.ConsumerId) == 0 {
		return ErrEmptyConsumerId
	}
	if len(m.ConsumerName) == 0 {
		return ErrEmptyConsumerName
	}
	if len(m.ConsumerDescription) == 0 {
		return ErrEmptyConsumerDescription
	}
	if m.ConsumerMaxMultiStakedFps < 2 {
		return ErrInvalidMaxMultiStakedFps
	}
	return nil
}

func NewCosmosConsumerRegister(consumerId, consumerName, consumerDescription string, consumerMaxMultiStakedFps uint32) *ConsumerRegister {
	return &ConsumerRegister{
		ConsumerId:                consumerId,
		ConsumerName:              consumerName,
		ConsumerDescription:       consumerDescription,
		ConsumerMaxMultiStakedFps: consumerMaxMultiStakedFps,
		ConsumerMetadata: &ConsumerRegister_CosmosConsumerMetadata{
			CosmosConsumerMetadata: &CosmosConsumerMetadata{},
		},
	}
}

func NewRollupConsumerRegister(consumerId, consumerName, consumerDescription string, rollupFinalityContractAddress string, consumerMaxMultiStakedFps uint32) *ConsumerRegister {
	return &ConsumerRegister{
		ConsumerId:                consumerId,
		ConsumerName:              consumerName,
		ConsumerDescription:       consumerDescription,
		ConsumerMaxMultiStakedFps: consumerMaxMultiStakedFps,
		ConsumerMetadata: &ConsumerRegister_RollupConsumerMetadata{
			RollupConsumerMetadata: &RollupConsumerMetadata{
				FinalityContractAddress: rollupFinalityContractAddress,
			},
		},
	}
}

func (cr *ConsumerRegister) Type() ConsumerType {
	if _, ok := cr.ConsumerMetadata.(*ConsumerRegister_CosmosConsumerMetadata); ok {
		return ConsumerType_COSMOS
	}
	return ConsumerType_ROLLUP
}

func (cr *ConsumerRegister) ToResponse() *ConsumerRegisterResponse {
	resp := &ConsumerRegisterResponse{
		ConsumerId:                cr.ConsumerId,
		ConsumerName:              cr.ConsumerName,
		ConsumerDescription:       cr.ConsumerDescription,
		ConsumerMaxMultiStakedFps: cr.ConsumerMaxMultiStakedFps,
	}
	if cr.ConsumerMetadata != nil {
		switch md := cr.ConsumerMetadata.(type) {
		case *ConsumerRegister_CosmosConsumerMetadata:
			resp.CosmosChannelId = md.CosmosConsumerMetadata.ChannelId
		case *ConsumerRegister_RollupConsumerMetadata:
			resp.RollupFinalityContractAddress = md.RollupConsumerMetadata.FinalityContractAddress
		}
	}
	return resp
}

func (cr *ConsumerRegister) Validate() error {
	if len(cr.ConsumerId) == 0 {
		return ErrEmptyConsumerId
	}
	if len(cr.ConsumerName) == 0 {
		return ErrEmptyConsumerName
	}
	if len(cr.ConsumerDescription) == 0 {
		return ErrEmptyConsumerDescription
	}
	if cr.ConsumerMaxMultiStakedFps < 2 {
		return ErrInvalidMaxMultiStakedFps
	}
	return nil
}
