//go:build !mainnet && !testnet

package allowlist

// This is a placeholder for the multi-staking allow list and allow list expiration height.
// Values here are not used in production and are only for testing purposes.
// The actual allow list is defined in the files with build tags:
// multi_staking_allow_list_mainnet.go and multi_staking_allow_list_testnet.go
// Make sure to include the correct tx hashes for the
// mainnet or testnet as needed in the corresponding
// files with the build tags
const multiStakingAllowListExpirationHeight = 5
const multiStakingAllowList = `{
  "tx_hashes": [
    "11f29d946c10d7774ce5c1732f51542171c09aedc6dc6f9ec1dcc68118fbe549",
    "ffc5e728e9c6c961f045b60833b6ebe0e22780b33ac359dac99fd0216a785508",
    "ffc5e728e9c6c961f045b60833b6ebe0e22780b33ac359dac99fd0216a785508"
  ]
}
`
// NOTE: Keep a duplicate tx hash in the list to test for duplicate handling