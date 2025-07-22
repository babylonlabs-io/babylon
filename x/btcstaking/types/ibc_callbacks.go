package types

const (
	// CallbackActionAddBsnRewardsMemo is the memo string indicating BSN reward distribution
	CallbackActionAddBsnRewardsMemo = "add_bsn_rewards"
)

// CallbackMemo defines the structure for callback memo in IBC transfers
type CallbackMemo struct {
	Action        string                 `json:"action,omitempty"`
	AddBsnRewards *CallbackAddBsnRewards `json:"add_bsn_rewards,omitempty"`
}

// CallbackAddBsnRewards callback memo information wrapper to
// add BSN rewards.
type CallbackAddBsnRewards struct {
	BsnConsumerID string    `json:"bsn_consumer_id"`
	FpRatios      []FpRatio `json:"fp_ratios"`
}
