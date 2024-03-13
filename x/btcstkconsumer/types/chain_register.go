package types

import (
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgRegisterChain{}

// Validate validates the set of params
func (cr *ChainRegister) Validate() error {
	if len(cr.ChainId) == 0 {
		return fmt.Errorf("ChainId must be non-empty")
	}
	if len(cr.ChainName) == 0 {
		return fmt.Errorf("ChainName must be non-empty")
	}

	return nil
}
