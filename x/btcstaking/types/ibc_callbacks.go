package types

import (
	"cosmossdk.io/math"
	"github.com/babylonlabs-io/babylon/v3/types"
)

const (
	// CallbackActionAddBsnRewardsMemo is the memo string indicating BSN reward distribution
	CallbackActionAddBsnRewardsMemo = "add_bsn_rewards"
)

// CallbackMemo defines the structure for callback memo in IBC transfers
type CallbackMemo struct {
	Action string `json:"action,omitempty"`
	// DestCallback mandatory dest_callback to call callbacks.
	DestCallback *CallbackInfo `json:"dest_callback,omitempty"`
}

// CallbackInfo contains the callback information
type CallbackInfo struct {
	// Address mandatory address to call callbacks, but unused
	Address       string                 `json:"address"`
	AddBsnRewards *CallbackAddBsnRewards `json:"add_bsn_rewards,omitempty"`
}

// CallbackAddBsnRewards callback memo information wrapper to
// add BSN rewards.
type CallbackAddBsnRewards struct {
	BsnConsumerID string                         `json:"bsn_consumer_id"`
	FpRatios      []CallbackAddBsnRewardsFpRatio `json:"fp_ratios"`
}

type CallbackAddBsnRewardsFpRatio struct {
	BtcPkHex string `json:"btc_pk_hex"`
	RatioDec string `json:"ratio"`
}

func (c *CallbackAddBsnRewards) ToFpRatios() ([]FpRatio, error) {
	fpRatios := make([]FpRatio, len(c.FpRatios))
	for i, r := range c.FpRatios {
		fpRatio, err := r.ToFpRatio()
		if err != nil {
			return nil, err
		}
		fpRatios[i] = fpRatio
	}
	return fpRatios, nil
}

func (c *CallbackAddBsnRewardsFpRatio) ToFpRatio() (FpRatio, error) {
	btcPk, err := types.NewBIP340PubKeyFromHex(c.BtcPkHex)
	if err != nil {
		return FpRatio{}, err
	}
	ratio, err := math.LegacyNewDecFromStr(c.RatioDec)
	if err != nil {
		return FpRatio{}, err
	}

	return FpRatio{
		BtcPk: btcPk,
		Ratio: ratio,
	}, nil
}
