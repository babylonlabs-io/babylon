package types

const (
	// CallbackActionAddBsnRewardsMemo is the memo string indicating BSN reward distribution
	CallbackActionAddBsnRewardsMemo = "add_bsn_rewards"
)

// CallbackMemo defines the structure for callback memo in IBC transfers
type CallbackMemo struct {
	Action string `json:"action,omitempty"`
	// DestCallback mandatory dest_callback wrapper to call contract callbacks.
	DestCallback *CallbackInfo `json:"dest_callback,omitempty"`
}

// CallbackInfo contains the callback information
type CallbackInfo struct {
	// Address mandatory address to call callbacks, but unused
	Address string `json:"address"`
	// AddBsnRewards fill out this field to call the action to give out
	// rewards to BSN using IBC callback
	AddBsnRewards *CallbackAddBsnRewards `json:"add_bsn_rewards,omitempty"`
}

// CallbackAddBsnRewards callback memo information wrapper to
// add BSN rewards.
type CallbackAddBsnRewards struct {
	BsnConsumerID string    `json:"bsn_consumer_id"`
	FpRatios      []FpRatio `json:"fp_ratios"`
}
