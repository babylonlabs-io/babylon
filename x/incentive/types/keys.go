package types

import (
	"cosmossdk.io/collections"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
)

const (
	// ModuleName defines the module name
	ModuleName = "incentive"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// RouterKey defines the module's message routing key
	RouterKey = ModuleName

	// MemStoreKey defines the in-memory store key
	MemStoreKey = "mem_incentive"
)

var (
	ParamsKey                                  = []byte{0x01}              // key prefix for the parameters
	BTCStakingGaugeKey                         = []byte{0x02}              // key prefix for BTC staking gauge at each height
	DelegatorWithdrawAddrPrefix                = []byte{0x03}              // key for delegator withdraw address
	RewardGaugeKey                             = []byte{0x04}              // key prefix for reward gauge for a given stakeholder in a given type
	RefundableMsgKeySetPrefix                  = collections.NewPrefix(5)  // key prefix for refundable msg key set
	FinalityProviderCurrentRewardsKeyPrefix    = collections.NewPrefix(6)  // key prefix for storing the Current rewards of finality provider by addr
	FinalityProviderHistoricalRewardsKeyPrefix = collections.NewPrefix(7)  // key prefix for storing the Historical rewards of finality provider by addr and period
	BTCDelegationRewardsTrackerKeyPrefix       = collections.NewPrefix(8)  // key prefix for BTC delegation rewards tracker info (del,fp) => BTCDelegationRewardsTracker
	BTCDelegatorToFPKey                        = []byte{0x9}               // key prefix for storing the map reference from delegation to finality provider (del) => fp
	RewardTrackerEvents                        = collections.NewPrefix(10) // key prefix for events of update in the voting power of BTC delegations (babylon block height) => []EventsPowerUpdateAtHeight
	RewardTrackerEventsLastProcessedHeight     = collections.NewPrefix(11) // key prefix for last processed block height of reward tracker events
	FPDirectGaugeKey                           = []byte{0x12}              // key prefix for FP direct rewards gauge at each height
)

// GetWithdrawAddrKey creates the key for a delegator's withdraw addr.
func GetWithdrawAddrKey(delAddr sdk.AccAddress) []byte {
	return append(DelegatorWithdrawAddrPrefix, address.MustLengthPrefix(delAddr.Bytes())...)
}
