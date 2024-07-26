package types

import (
	"context"

	"github.com/babylonlabs-io/babylon/types"
)

// combine multiple finality hooks, all hook functions are run in array sequence
var _ FinalityHooks = &MultiFinalityHooks{}

type MultiFinalityHooks []FinalityHooks

func NewMultiFinalityHooks(hooks ...FinalityHooks) MultiFinalityHooks {
	return hooks
}

func (h MultiFinalityHooks) AfterSluggishFinalityProviderDetected(ctx context.Context, btcPk *types.BIP340PubKey) error {
	for i := range h {
		if err := h[i].AfterSluggishFinalityProviderDetected(ctx, btcPk); err != nil {
			return err
		}
	}

	return nil
}
