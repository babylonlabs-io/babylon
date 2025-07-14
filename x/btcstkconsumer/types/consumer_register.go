package types

import (
	"fmt"

	"cosmossdk.io/math"
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
	if m.BabylonCommission.IsNegative() {
		return fmt.Errorf("babylon commission cannot be negative")
	}
	if m.BabylonCommission.GT(math.LegacyOneDec()) {
		return fmt.Errorf("babylon commission cannot be greater than 1.0")
	}
	return nil
}

func NewCosmosConsumerRegister(consumerId, consumerName, consumerDescription string, babylonCommission math.LegacyDec) *ConsumerRegister {
	return &ConsumerRegister{
		ConsumerId:               consumerId,
		ConsumerName:             consumerName,
		ConsumerDescription:      consumerDescription,
		BabylonRewardsCommission: babylonCommission,
		ConsumerMetadata: &ConsumerRegister_CosmosConsumerMetadata{
			CosmosConsumerMetadata: &CosmosConsumerMetadata{},
		},
	}
}

func NewRollupConsumerRegister(consumerId, consumerName, consumerDescription string, rollupFinalityContractAddress string, babylonCommission math.LegacyDec) *ConsumerRegister {
	return &ConsumerRegister{
		ConsumerId:               consumerId,
		ConsumerName:             consumerName,
		ConsumerDescription:      consumerDescription,
		BabylonRewardsCommission: babylonCommission,
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
		ConsumerId:          cr.ConsumerId,
		ConsumerName:        cr.ConsumerName,
		ConsumerDescription: cr.ConsumerDescription,
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
	return nil
}
