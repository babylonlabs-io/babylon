package types

const (
	// Event types for incentive module
	// EventTypeBTCStakingReward is emitted when BTC staking rewards are accumulated
	EventTypeBTCStakingReward = "btc_staking_reward"

	// EventTypeFPDirectRewards is emitted when finality provider direct rewards are accumulated
	EventTypeFPDirectRewards = "fp_direct_rewards"

	// Event attribute keys
	// AttributeKeyAmount represents the amount of rewards
	AttributeKeyAmount = "amount"
)
