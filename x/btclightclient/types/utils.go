package types

import (
	"time"

	"github.com/btcsuite/btcd/chaincfg"
)

func BlocksPerRetarget(params *chaincfg.Params) int32 {
	targetTimespan := int64(params.TargetTimespan / time.Second)
	targetTimePerBlock := int64(params.TargetTimePerBlock / time.Second)
	return int32(targetTimespan / targetTimePerBlock)
}

func IsRetargetBlock(info *BTCHeaderInfo, params *chaincfg.Params) bool {
	blocksPerRetarget := BlocksPerRetarget(params)
	if blocksPerRetarget < 0 {
		panic("Invalid blocks per retarget value")
	}
	return info.Height%uint32(blocksPerRetarget) == 0
}
