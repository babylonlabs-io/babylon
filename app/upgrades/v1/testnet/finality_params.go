package testnet

// The reason finality parameters are in the upgrade is because its structure
// had an update and it is possible to overwrite during the upgrade.
const FinalityParamStr = `{
  "max_active_finality_providers": 100,
  "signed_blocks_window": 10000,
  "finality_sig_timeout": 3,
  "min_signed_per_window": "0.05",
  "min_pub_rand": 500,
  "jail_duration": "3600s",
  "finality_activation_height": 200
}`
