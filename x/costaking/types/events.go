package types

import sdk "github.com/cosmos/cosmos-sdk/types"

const (
	// Event types for costaking module
	// EventTypeValidatorDirectRewards is emitted when direct validator rewards from costaking are allocated
	EventTypeValidatorDirectRewards = "validator_direct_rewards"

	// Event attribute keys
	// AttributeKeyAmount represents the amount of rewards
	AttributeKeyAmount = "amount"
	// AttributeKeyValidatorCount represents the number of validators that received rewards
	AttributeKeyValidatorCount = "validator_count"
)

func NewEventCostakersAddRewards(rewardsToAdd sdk.Coins, currentRwd CurrentRewards) EventCostakersAddRewards {
	return EventCostakersAddRewards{
		AddRewards:        rewardsToAdd,
		CurrentRewards:    currentRwd.Rewards,
		CurrentPeriod:     currentRwd.Period,
		CurrentTotalScore: currentRwd.TotalScore,
	}
}
