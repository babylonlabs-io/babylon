//go:build !mainnet && !testnet

package types

// This is a placeholder for the multi-staking allow list and allow list expiration height.
// NOTE: keep it always empty to avoid confusion.
// The actual allow list is defined in the files with build tags:
// multi_staking_allow_list_mainnet.go and multi_staking_allow_list_testnet.go
// Make sure to include the correct tx hashes for the
// mainnet or testnet as needed in the corresponding
// files with the build tags
const multiStakingAllowListExpirationHeight = 0
const multiStakingAllowList = `{
  "tx_hashes": []
}
`
