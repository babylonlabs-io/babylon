package types

const (
	// Event types for coostaking module
	// EventTypeValidatorDirectRewards is emitted when direct validator rewards from coostaking are allocated
	EventTypeValidatorDirectRewards = "validator_direct_rewards"

	// Event attribute keys
	// AttributeKeyAmount represents the amount of rewards
	AttributeKeyAmount = "amount"
	// AttributeKeyValidatorCount represents the number of validators that received rewards
	AttributeKeyValidatorCount = "validator_count"
)
