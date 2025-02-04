package types

import (
	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func HashMsg(msg sdk.Msg) []byte {
	msgBytes := ModuleCdc.MustMarshal(msg)
	msgHash := tmhash.Sum(msgBytes)
	return msgHash
}

// ToRewardGauge parses to RewardGauge
func (rgr RewardGaugesResponse) ToRewardGauge() RewardGauge {
	return RewardGauge(rgr)
}
