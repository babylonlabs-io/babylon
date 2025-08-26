package types

import (
	"fmt"
	"unicode/utf8"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.HasValidateBasic = (*MsgRegisterConsumer)(nil)
var minBabylonRewardsCommission = math.LegacyMustNewDecFromStr("0.01") // 1% minimum

func (m *MsgRegisterConsumer) ValidateBasic() error {
	if len(m.ConsumerId) == 0 {
		return fmt.Errorf("ConsumerId must be non-empty")
	}
	if !utf8.ValidString(m.ConsumerId) {
		return fmt.Errorf("ConsumerId must be valid UTF-8")
	}
	if len(m.ConsumerName) == 0 {
		return fmt.Errorf("ConsumerName must be non-empty")
	}
	if len(m.ConsumerDescription) == 0 {
		return fmt.Errorf("ConsumerDescription must be non-empty")
	}
	return ValidateBbnCommission(m.BabylonRewardsCommission)
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
		ConsumerId:               cr.ConsumerId,
		ConsumerName:             cr.ConsumerName,
		ConsumerDescription:      cr.ConsumerDescription,
		BabylonRewardsCommission: cr.BabylonRewardsCommission,
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
	return ValidateBbnCommission(cr.BabylonRewardsCommission)
}

func ValidateBbnCommission(bbnCommission math.LegacyDec) error {
	if bbnCommission.IsNil() {
		return fmt.Errorf("babylon commission cannot be nil")
	}
	if bbnCommission.IsNegative() {
		return fmt.Errorf("babylon commission cannot be negative")
	}
	if bbnCommission.GT(math.LegacyOneDec()) {
		return fmt.Errorf("babylon commission cannot be greater than 1.0")
	}
	if bbnCommission.LT(minBabylonRewardsCommission) {
		return fmt.Errorf("babylon commission cannot be less than %s", minBabylonRewardsCommission)
	}
	return nil
}
