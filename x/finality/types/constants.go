package types

const (
	// Setting max amount of finalized and rewarded blocks per EndBlock to 10000,
	// with this setting, block processing times are <1s, even if BTC finality
	// stalls a lot
	MaxFinalizedRewardedBlocksPerEndBlock = uint64(10000)
)
