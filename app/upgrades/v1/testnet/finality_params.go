package testnet

// TODO Some default parameters. Consider how to switch those depending on network:
// mainnet, testnet, devnet etc.
const FinalityParamStr = `{
  "signed_blocks_window": 100,
  "finality_sig_timeout": 3,
  "min_signed_per_window": "0.1",
  "min_pub_rand": 100,
  "jail_duration": "86400s"
}`
