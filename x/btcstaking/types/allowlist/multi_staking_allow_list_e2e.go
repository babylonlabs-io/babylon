//go:build testnet && e2e_upgrade

package allowlist

// Values here are not used in production and are only used for e2e tests.
// The actual allow list is defined in the files with build tags:
// multi_staking_allow_list_mainnet.go and multi_staking_allow_list_testnet.go
// Make sure to include the correct tx hashes for the
// mainnet or testnet as needed in the corresponding
// files with the build tags
const multiStakingAllowListExpirationHeight = 0 // Keep this at 0 for e2e tests

const multiStakingAllowList = `{
  "tx_hashes": [
    "11f29d946c10d7774ce5c1732f51542171c09aedc6dc6f9ec1dcc68118fbe549",
    "29dfed14500522ce56389745156776823f459639ac741ebedad51acd3a67b5f8",
    "ffc5e728e9c6c961f045b60833b6ebe0e22780b33ac359dac99fd0216a785508"
  ]
}
`
