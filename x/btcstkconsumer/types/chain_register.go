package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgRegisterConsumer{}

// Validate validates the set of params
func (cr *ConsumerRegister) Validate() error {
	if len(cr.ConsumerId) == 0 {
		return fmt.Errorf("ConsumerId must be non-empty")
	}
	if len(cr.ConsumerName) == 0 {
		return fmt.Errorf("ConsumerName must be non-empty")
	}

	return nil
}
