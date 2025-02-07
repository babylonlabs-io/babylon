package mainnet

// TODO Some default parameters
const BtcStakingParamsStr = `[
  {
    "covenant_pks": [
      "43311589af63c2adda04fcd7792c038a05c12a4fe40351b3eb1612ff6b2e5a0e",
      "d415b187c6e7ce9da46ac888d20df20737d6f16a41639e68ea055311e1535dd9",
      "d27cd27dbff481bc6fc4aa39dd19405eb6010237784ecba13bab130a4a62df5d",
      "a3e107fee8879f5cf901161dbf4ff61c252ba5fec6f6407fe81b9453d244c02c",
      "c45753e856ad0abb06f68947604f11476c157d13b7efd54499eaa0f6918cf716"
    ],
    "covenant_quorum": 3,
    "min_staking_value_sat": 10000,
    "max_staking_value_sat": 10000000000,
    "min_staking_time_blocks": 10,
    "max_staking_time_blocks": 65535,
    "slashing_pk_script": "dqkUAQEBAQEBAQEBAQEBAQEBAQEBAQGIrA==",
    "min_slashing_tx_fee_sat": 1000,
    "slashing_rate": "0.100000000000000000",
    "unbonding_time_blocks": 101,
    "unbonding_fee_sat": 1000,
    "min_commission_rate": "0.03",
    "delegation_creation_base_gas_fee": 1000,
    "allow_list_expiration_height": 0,
    "btc_activation_height": 100
  }
]`
