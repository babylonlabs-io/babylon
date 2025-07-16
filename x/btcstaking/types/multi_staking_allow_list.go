package types

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
)

type multiStakingAllowListData struct {
	TxHashes []string `json:"tx_hashes"`
}

func LoadMultiStakingAllowList() ([]*chainhash.Hash, error) {
	buff := bytes.NewBufferString(multiStakingAllowList)

	var d multiStakingAllowListData
	err := json.Unmarshal(buff.Bytes(), &d)
	if err != nil {
		return nil, err
	}

	allowedTxHashes := make([]*chainhash.Hash, len(d.TxHashes))
	for i, txHash := range d.TxHashes {
		hash, err := chainhash.NewHashFromStr(txHash)
		if err != nil {
			return nil, fmt.Errorf("failed to parse tx hash: %w", err)
		}
		allowedTxHashes[i] = hash
	}

	return allowedTxHashes, nil
}

// IsMultiStakingAllowListEnabled checks if the allow list is enabled at the given height
// allow list is enabled if multiStakingAllowListExpirationHeight is larger than 0,
// and current block height is less than multiStakingAllowListExpirationHeight
func IsMultiStakingAllowListEnabled(height int64) bool {
	return multiStakingAllowListExpirationHeight > 0 && height < multiStakingAllowListExpirationHeight
}
