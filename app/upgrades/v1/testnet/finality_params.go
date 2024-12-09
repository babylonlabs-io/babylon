package testnet

// The reason finality parameters are in the upgrade is because its structure
// had an update and it is possible to overwrite during the upgrade.
// The finality activation height is when the FPs need to have their
// programs ready to start sending finality signatures and it is defined
// to be 48 hours after the upgrade, the upgrade will happen close to 
// block 200 and 48 hours of blocks with 10 seconds block time gives 17280 
// blocks. In this case the finality activation block heigth will be set
// as 17480 = 17280 (48hrs worth of blocks) + ~200 (blocks of TGE).
const FinalityParamStr = `{
  "max_active_finality_providers": 100,
  "signed_blocks_window": 100,
  "finality_sig_timeout": 3,
  "min_signed_per_window": "0.1",
  "min_pub_rand": 500,
  "jail_duration": "86400s",
  "finality_activation_height": 17480
}`
