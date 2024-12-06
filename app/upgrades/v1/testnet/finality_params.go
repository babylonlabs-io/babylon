package testnet

// The finality parameters are in the upgrade just because his structure
// had an update and it is possible to overwrite during the upgrade.
// The finality activation height is when the FP need to have their
// program ready to start send finality signatures and it could be
// the same block height where the allow list is expired in this case
// babylon block height: 26120
const FinalityParamStr = `{
  "max_active_finality_providers": 100,
  "signed_blocks_window": 100,
  "finality_sig_timeout": 3,
  "min_signed_per_window": "0.1",
  "min_pub_rand": 100,
  "jail_duration": "86400s",
  "finality_activation_height": 26120
}`
