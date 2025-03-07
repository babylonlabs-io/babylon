package testnet

// The BTC staking parameters should be compatible with the global-parameters
// used in the BTC staking caps on bbn-test-4
// ref: https://github.com/babylonchain/networks/blob/main/bbn-test-4/parameters/global-params.json
// allow_list_expiration_height set to close to 72 hours of blocks 25920 + ~200 blocks of TGE
// producing each babylon block in 10 seconds = 26120.
// In global params version 2,3,4 we inserted the cap_height: 201385, at the moment we don't
// have a good way of specifyng the cap to avoid overflow to join as valid BTC staking,
// so this will be filtered out by the allow list of transactions
// ./allowed_staking_tx_hashes.go
const BtcStakingParamsStr = `[
  {
    "covenant_pks": [
      "8c1716ebbc505e842107137501e9d10271a32085e04ae3be8cff5d3edc4b83c9",
      "92c3409b169508129e2cfdd13727e8fd882e829998e27f86f90b37033ff2f002",
      "aa357bc493c51e7d413f99babdc128d2140d3a6bcf39859bd52a417bc0f8d4e9"
    ],
    "covenant_quorum": 2,
    "min_staking_value_sat": 20000,
    "max_staking_value_sat": 50000,
    "min_staking_time_blocks": 64000,
    "max_staking_time_blocks": 64000,
    "slashing_pk_script": "dqkUAQEBAQEBAQEBAQEBAQEBAQEBAQGIrA==",
    "min_slashing_tx_fee_sat": 1000,
    "slashing_rate": "0.100000000000000000",
    "unbonding_time_blocks": 144,
    "unbonding_fee_sat": 3000,
    "min_commission_rate": "0.03",
    "delegation_creation_base_gas_fee": 1000,
    "allow_list_expiration_height": 17484,
    "btc_activation_height": 886254
  },
  {
    "covenant_pks": [
      "8c1716ebbc505e842107137501e9d10271a32085e04ae3be8cff5d3edc4b83c9",
      "92c3409b169508129e2cfdd13727e8fd882e829998e27f86f90b37033ff2f002",
      "aa357bc493c51e7d413f99babdc128d2140d3a6bcf39859bd52a417bc0f8d4e9"
    ],
    "covenant_quorum": 2,
    "min_staking_value_sat": 20000,
    "max_staking_value_sat": 51000,
    "min_staking_time_blocks": 64000,
    "max_staking_time_blocks": 64000,
    "slashing_pk_script": "dqkUAQEBAQEBAQEBAQEBAQEBAQEBAQGIrA==",
    "min_slashing_tx_fee_sat": 1000,
    "slashing_rate": "0.100000000000000000",
    "unbonding_time_blocks": 144,
    "unbonding_fee_sat": 3000,
    "min_commission_rate": "0.03",
    "delegation_creation_base_gas_fee": 1000,
    "allow_list_expiration_height": 17484,
    "btc_activation_height": 886581
  },
  {
    "covenant_pks": [
      "8c1716ebbc505e842107137501e9d10271a32085e04ae3be8cff5d3edc4b83c9",
      "92c3409b169508129e2cfdd13727e8fd882e829998e27f86f90b37033ff2f002",
      "aa357bc493c51e7d413f99babdc128d2140d3a6bcf39859bd52a417bc0f8d4e9",
      "2a9dfda4cf7f898a814cc01da112e544fcc8a9dfac33b8b68f67f82de28155ff",
      "b2988455c7e8a755a2dd43c8591d41e29cc44fe9392993f0f31ec464b4268a18",
      "2f8bcf7ce6e9cca371ebbe64ee479fdba8796e9b87143ee0732370ea253fd02d",
      "c2502f5e5d1b6c9700883818da721e8e17f542afa0634319815926788f51bb7a",
      "73ee6f6275f87f737eb680f63f4b66ce05f574b9b47dcd2f9c7dde726cc72464",
      "50602832ccb15f55a14609824450f707d3bab6731c19dea115e83c3a90c85edb"
    ],
    "covenant_quorum": 6,
    "min_staking_value_sat": 20000,
    "max_staking_value_sat": 51000,
    "min_staking_time_blocks": 64000,
    "max_staking_time_blocks": 64000,
    "slashing_pk_script": "ABQhq763Fas0jS55PAKSFLw+0zlB4w==",
    "min_slashing_tx_fee_sat": 1000,
    "slashing_rate": "0.05",
    "unbonding_time_blocks": 144,
    "unbonding_fee_sat": 3000,
    "min_commission_rate": "0.03",
    "delegation_creation_base_gas_fee": 1095000,
    "allow_list_expiration_height": 17484,
    "btc_activation_height": 887000
  }
]`
