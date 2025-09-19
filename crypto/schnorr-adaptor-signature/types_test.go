package schnorr_adaptor_signature_test

import (
	"fmt"
	"testing"

	"github.com/babylonlabs-io/babylon/v4/btcstaking"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	bstypes "github.com/babylonlabs-io/babylon/v4/x/btcstaking/types"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/require"
)

var (
	// https://babylon-testnet-api.nodes.guru/babylon/btcstaking/v1/btc_delegation/56d54aa3f925a86f51da8c791ab7251ed4a62ac0ec187ac50f5e50363039b880

	btcDelegationJSON = `
	{
    "staker_addr": "bbn1h4d84m77wtuklrv8jgryv5e748ke6mz5k530uk",
    "btc_pk": "4b8a3b2bb1c4cc893c4e735ead3a3480db4c40b7383e12b39a1c19d72ff29f1d",
    "fp_btc_pk_list": [
      "94c0d27c2015db39a3a0ec9457137e2d52b4b29188c95967d226017860454abe"
    ],
    "staking_time": 64000,
    "start_height": 0,
    "end_height": 0,
    "total_sat": "50000",
    "staking_tx_hex": "02000000012c1ca601b81bf5bdd97081d1bf17241d4d688f51ccbe8be3d3f3174d0e4e4aa40100000000ffffffff0250c3000000000000225120d0d55103aa70a12162f733805c3a2f5ff8e857d5fc92381c3d6f22a791165ac115400f00000000002251206f5ec73002ee8b5b2bb942f26e169354821e6ec06f9b3a1d3cf355d6f276c5d800000000",
    "slashing_tx_hex": "020000000180b8393036505e0fc57a18ecc02aa6d41e25b71a798cda516fa825f9a34ad5560000000000ffffffff02c4090000000000001600145be12624d08a2b424095d7c07221c33450d14bf11ca2000000000000225120551533c53731ef30e5ee446cbabce558db154df73c3a28f48597742760fa884300000000",
    "delegator_slash_sig_hex": "3fd5d1e6494d25717dd34a5557315e35bdb49645aec32acde1d06a71aac6fbb4439cfe57c9c6dbf39bf36308802c4a1fb7d4a8858a4592376943b09331496b68",
    "covenant_sigs": [
      {
        "cov_pk": "3bb93dfc8b61887d771f3630e9a63e97cbafcfcc78556a474df83a31a0ef899c",
        "adaptor_sigs": [
          "AjDPRxeQPNrumcw+R2D0YrWq/yMgmXmXcms5PuMdBKxG4sb2X1/Gg2LaANrSqrHXJ3AlQ1MQmAknU9MvIBaIdeQB"
        ]
      },
      {
        "cov_pk": "17921cf156ccb4e73d428f996ed11b245313e37e27c978ac4d2cc21eca4672e4",
        "adaptor_sigs": [
          "AuKnYX3BTsCPjxXbgaVd8CN/bDTpy397bVRKuhd44lRPxzeIP5Wq1qiTgHASx2SPdAWtGMVtMZn/lYXozoxJDrUA"
        ]
      },
      {
        "cov_pk": "113c3a32a9d320b72190a04a020a0db3976ef36972673258e9a38a364f3dc3b0",
        "adaptor_sigs": [
          "Ai8QmVljb/6GN+OdwlhmgKM2MI+UVrJgE3NTAHmiKyXMsLmKnBxdCAKeYxsxGo0Q6EWKshlj3O0SsE3qVPncT/QA"
        ]
      },
      {
        "cov_pk": "0aee0509b16db71c999238a4827db945526859b13c95487ab46725357c9a9f25",
        "adaptor_sigs": [
          "ApmNkGk4VMfAc0Jk+8UvjxEnwupmdOLZznatzOVaj8bJLakjhQNkqA6mMzlrMamm652rwHgmTV7a56C3i09YAEYB"
        ]
      },
      {
        "cov_pk": "fa9d882d45f4060bdb8042183828cd87544f1ea997380e586cab77d5fd698737",
        "adaptor_sigs": [
          "AoUdt/DJa6CT7w4U3B9E/CkV9WuyM9AyHPT9KxcmSscqNDQSdlcyLZ4RVNp2bVynp/AJ5GQQVh7N5/TtnkAysk8A"
        ]
      },
      {
        "cov_pk": "f5199efae3f28bb82476163a7e458c7ad445d9bffb0682d10d3bdb2cb41f8e8e",
        "adaptor_sigs": [
          "Aix71oXRcy4wM8f2Ao3ec3GWtByKtDvLTKq53hzuIuPy5Uttkzu0XxBOPpHmzrezuxSEpfhTtJ09dpzvgaueuhUA"
        ]
      }
    ],
    "staking_output_idx": 0,
    "active": false,
    "status_desc": "VERIFIED",
    "unbonding_time": 1008,
    "undelegation_response": {
      "unbonding_tx_hex": "020000000180b8393036505e0fc57a18ecc02aa6d41e25b71a798cda516fa825f9a34ad5560000000000ffffffff0180bb00000000000022512041ac9fef64bf37b9f610df51100a12d30b2169e593dffd3d9d132231cc456e5100000000",
      "covenant_unbonding_sig_list": [
        {
          "pk": "3bb93dfc8b61887d771f3630e9a63e97cbafcfcc78556a474df83a31a0ef899c",
          "sig": "wtuUiDqBI2Ms5tsFhsVThqYvQj5EFPg9yZXw5E/H1sWH5AQxuFAD9ITmOTlNl5KdKsM5yYyVmVuz2ZLU+atJpw=="
        },
        {
          "pk": "17921cf156ccb4e73d428f996ed11b245313e37e27c978ac4d2cc21eca4672e4",
          "sig": "c7GW0wGR3IUkzyRed8nCHodVk7YK8TG6Asz/qk0tfiMeQbrZQ+3qccIvCeltLvBZssLaREAWqXOkdx/wfkatGw=="
        },
        {
          "pk": "113c3a32a9d320b72190a04a020a0db3976ef36972673258e9a38a364f3dc3b0",
          "sig": "CHOf/jF5u8cWWzpKCDO5uenzHFFgT6DyE2sK5cFt/oaezDFMaggexkQpw4aemyCbQJtMaRPYRi/KDx6kTjrhtw=="
        },
        {
          "pk": "0aee0509b16db71c999238a4827db945526859b13c95487ab46725357c9a9f25",
          "sig": "nMYw0S/iwoYjBxVMihPjNH+NFPJzquZUtfbCqprb+w3o6m+sRyl6R2V+SysjPn4ueoZx3am5DS3ylHCpbOVOsg=="
        },
        {
          "pk": "fa9d882d45f4060bdb8042183828cd87544f1ea997380e586cab77d5fd698737",
          "sig": "9GziiiEc4N1uDfWtK5bQzBa2hzQWKUXvbrLyS5F/St1Qde18V7jL3xFNsRftxkhrQ4/Q+vESB/S+ewDxl/g6eQ=="
        },
        {
          "pk": "f5199efae3f28bb82476163a7e458c7ad445d9bffb0682d10d3bdb2cb41f8e8e",
          "sig": "1HiBOm4nC9vcU6s+88Ze3iNm/oiP+S6j/UxHelwhqlV9YyedvaVAKkwYpDJR+7yoOUTqA/4YKmBS/fcaC45MNg=="
        }
      ],
      "slashing_tx_hex": "0200000001c4841deb25c002d89d170041c429f5b96ed0c5481dd31ec578253693e5974b380000000000ffffffff0260090000000000001600145be12624d08a2b424095d7c07221c33450d14bf1b09a000000000000225120551533c53731ef30e5ee446cbabce558db154df73c3a28f48597742760fa884300000000",
      "delegator_slashing_sig_hex": "d31d10f4a708b0a77473d8c2ee55ad5ba78cdaef3fc443699abe9dd8636cfa5edb6f67c83488e3276845a980c3ee08db0f7a4da46cf55b1ce831622e92e57638",
      "covenant_slashing_sigs": [
        {
          "cov_pk": "3bb93dfc8b61887d771f3630e9a63e97cbafcfcc78556a474df83a31a0ef899c",
          "adaptor_sigs": [
            "AiWPX3h8wXuzeanjcep2xwgeGfMbXx7QDgDaDP8sZ1UvXWzSuRx3/I3yzNIaxXWzqGYwqZgGgY+YoVviw8kJHOYB"
          ]
        },
        {
          "cov_pk": "17921cf156ccb4e73d428f996ed11b245313e37e27c978ac4d2cc21eca4672e4",
          "adaptor_sigs": [
            "AqJ1cVbt05qWDE+Y2JwRTc8+aNx7V+jIhNbzXLcJMbDoTLPRPfylF3VHQexaauGKKEDwHmHODIllPmxRfB9h/6cB"
          ]
        },
        {
          "cov_pk": "113c3a32a9d320b72190a04a020a0db3976ef36972673258e9a38a364f3dc3b0",
          "adaptor_sigs": [
            "AlSRKXmPnYY4wPJuUn4i0dq8ZEvwfI6Tr68E8sFtwJ61mYuofZx/TNF4T+loQK+RrmMm8RmWaV3ZUApggyLOhfwA"
          ]
        },
        {
          "cov_pk": "0aee0509b16db71c999238a4827db945526859b13c95487ab46725357c9a9f25",
          "adaptor_sigs": [
            "AjMOZubU2Ma+Az88MdXUMHkLChL3xTVFOHq6XtdbFOSw9w6FnScczwnhU5DrFWe7Q8hFIRVXwOESOg/pJ11f34AB"
          ]
        },
        {
          "cov_pk": "fa9d882d45f4060bdb8042183828cd87544f1ea997380e586cab77d5fd698737",
          "adaptor_sigs": [
            "AiS0WNatlygDfdfiHicxuQCGi22uIQzfyI4x1Yy1OEYLntRZqO3BLlaZY1BkRv5Zz20VjEZkMwkY7H8IXQkrF7AA"
          ]
        },
        {
          "cov_pk": "f5199efae3f28bb82476163a7e458c7ad445d9bffb0682d10d3bdb2cb41f8e8e",
          "adaptor_sigs": [
            "As8dnXQ6onlUussxV4iZkrP3LU3yvmm0V5A+pNaTm+ur4hnjSwRsJSBYA260Y1FNfuvqOHy5x3dzECWUCxj4Sy4B"
          ]
        }
      ],
      "delegator_unbonding_info_response": null
    },
    "params_version": 6
}
	`

	// https://babylon-testnet-api.nodes.guru/babylon/btcstaking/v1/params
	paramsJSON = `
	{
    "covenant_pks": [
      "fa9d882d45f4060bdb8042183828cd87544f1ea997380e586cab77d5fd698737",
      "0aee0509b16db71c999238a4827db945526859b13c95487ab46725357c9a9f25",
      "17921cf156ccb4e73d428f996ed11b245313e37e27c978ac4d2cc21eca4672e4",
      "113c3a32a9d320b72190a04a020a0db3976ef36972673258e9a38a364f3dc3b0",
      "79a71ffd71c503ef2e2f91bccfc8fcda7946f4653cef0d9f3dde20795ef3b9f0",
      "3bb93dfc8b61887d771f3630e9a63e97cbafcfcc78556a474df83a31a0ef899c",
      "d21faf78c6751a0d38e6bd8028b907ff07e9a869a43fc837d6b3f8dff6119a36",
      "40afaf47c4ffa56de86410d8e47baa2bb6f04b604f4ea24323737ddc3fe092df",
      "f5199efae3f28bb82476163a7e458c7ad445d9bffb0682d10d3bdb2cb41f8e8e"
    ],
    "covenant_quorum": 6,
    "min_staking_value_sat": "50000",
    "max_staking_value_sat": "35000000000",
    "min_staking_time_blocks": 10000,
    "max_staking_time_blocks": 64000,
    "slashing_pk_script": "ABRb4SYk0IorQkCV18ByIcM0UNFL8Q==",
    "min_slashing_tx_fee_sat": "6000",
    "slashing_rate": "0.050000000000000000",
    "unbonding_time_blocks": 1008,
    "unbonding_fee_sat": "2000",
    "min_commission_rate": "0.030000000000000000",
    "delegation_creation_base_gas_fee": "1095000",
    "allow_list_expiration_height": "26124",
    "btc_activation_height": 235952
}
	`
)

func TestOldFormatCompat(t *testing.T) {
	var btcDel bstypes.BTCDelegationResponse
	err := bstypes.ModuleCdc.UnmarshalJSON([]byte(btcDelegationJSON), &btcDel)
	require.NoError(t, err)

	var params bstypes.Params
	err = bstypes.ModuleCdc.UnmarshalJSON([]byte(paramsJSON), &params)
	require.NoError(t, err)

	stakingMsgTx, _, err := bbn.NewBTCTxFromHex(btcDel.StakingTxHex)
	require.NoError(t, err)
	fundingOut := stakingMsgTx.TxOut[btcDel.StakingOutputIdx]

	slashingMsgTx, _, err := bbn.NewBTCTxFromHex(btcDel.SlashingTxHex)
	require.NoError(t, err)
	slashingTx, err := bstypes.NewBTCSlashingTxFromMsgTx(slashingMsgTx)
	require.NoError(t, err)

	stakingInfo, err := getBTCStakingInfo(&btcDel, &params, &chaincfg.SigNetParams)
	require.NoError(t, err)
	slashingSpendInfo, err := stakingInfo.SlashingPathSpendInfo()
	require.NoError(t, err)

	for i := range btcDel.CovenantSigs {
		err = slashingTx.EncVerifyAdaptorSignatures(
			fundingOut,
			slashingSpendInfo,
			btcDel.CovenantSigs[i].CovPk,
			btcDel.FpBtcPkList,
			btcDel.CovenantSigs[i].AdaptorSigs,
		)
		require.NoError(t, err)
	}
}

func getBTCStakingInfo(d *bstypes.BTCDelegationResponse, bsParams *bstypes.Params, btcNet *chaincfg.Params) (*btcstaking.StakingInfo, error) {
	fpBtcPkList, err := bbn.NewBTCPKsFromBIP340PKs(d.FpBtcPkList)
	if err != nil {
		return nil, fmt.Errorf("failed to convert finality provider pks to BTC pks %v", err)
	}
	covenantBtcPkList, err := bbn.NewBTCPKsFromBIP340PKs(bsParams.CovenantPks)
	if err != nil {
		return nil, fmt.Errorf("failed to convert covenant pks to BTC pks %v", err)
	}
	stakingInfo, err := btcstaking.BuildStakingInfo(
		d.BtcPk.MustToBTCPK(),
		fpBtcPkList,
		covenantBtcPkList,
		bsParams.CovenantQuorum,
		uint16(d.StakingTime),
		btcutil.Amount(d.TotalSat),
		btcNet,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create BTC staking info: %v", err)
	}
	return stakingInfo, nil
}
