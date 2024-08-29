package signetlaunch

const SignedFPsStr = `{
  "signed_txs_create_fp": [
    {
      "body": {
        "messages": [
          {
            "@type": "/babylon.btcstaking.v1.MsgCreateFinalityProvider",
            "addr": "bbn1gwecky0m842kvjg7mcvt9z330rz96948aplqlm",
            "description": {
              "moniker": "fp-1",
              "identity": "",
              "website": "fp1.com.br",
              "security_contact": "fp.1@email.com",
              "details": "best-fp-1"
            },
            "commission": "0.050000000000000000",
            "btc_pk": "a94eef36ea7c596ba01b2018d55c202ebd8a82a0baf1a435818bc524bfd4e10a",
            "pop": {
              "btc_sig_type": "BIP340",
              "btc_sig": "q5aykrV4imao9kiIVvWhBf8hPbIQW7GbdnDDCAZQRyCm4ZBaUiAGO3oFbEgVeLKNAd0xGmcCAJSbXu0OrpcQkQ=="
            }
          }
        ],
        "memo": "",
        "timeout_height": "0",
        "extension_options": [],
        "non_critical_extension_options": []
      },
      "auth_info": {
        "signer_infos": [
          {
            "public_key": {
              "@type": "/cosmos.crypto.secp256k1.PubKey",
              "key": "A9XXtGMjEFgavPv7GHo5rbI/XulwA0Hn2xlzsdCRDCTl"
            },
            "mode_info": {
              "single": {
                "mode": "SIGN_MODE_DIRECT"
              }
            },
            "sequence": "0"
          }
        ],
        "fee": {
          "amount": [
            {
              "denom": "ubbn",
              "amount": "2000000"
            }
          ],
          "gas_limit": "200000",
          "payer": "",
          "granter": ""
        },
        "tip": null
      },
      "signatures": [
        "8wQGbPM5Xc5PmrynGmxbslqSA6tFW/5Vgg9sVZZ1WWsRTR5m040t2wgR+BjUrLNnO+JtrDlb38Su4XFSR76b9Q=="
      ]
    },
    {
      "body": {
        "messages": [
          {
            "@type": "/babylon.btcstaking.v1.MsgCreateFinalityProvider",
            "addr": "bbn1mwwywrmynkf0n5maps6yrtvgx2qqh3mlccdg6g",
            "description": {
              "moniker": "fp-2",
              "identity": "",
              "website": "fp2.com.br",
              "security_contact": "fp.2@email.com",
              "details": "best-fp-2"
            },
            "commission": "0.100000000000000000",
            "btc_pk": "bae0f3bfedc4de9e776fcbbb4b1dbae2641193fc20527ffc0a728968ebcd2d95",
            "pop": {
              "btc_sig_type": "BIP340",
              "btc_sig": "IFU+77I8e7VOGudJdN5kk/8Hs2Biqiiw+sejBbYrtPRSNhFRhFrxhOru5kYHPxZ2XwadlfZfZjmXB8Uvj/hb5w=="
            }
          }
        ],
        "memo": "",
        "timeout_height": "0",
        "extension_options": [],
        "non_critical_extension_options": []
      },
      "auth_info": {
        "signer_infos": [
          {
            "public_key": {
              "@type": "/cosmos.crypto.secp256k1.PubKey",
              "key": "A3XkUtvcp3DnAvDKN4zYkES3xc6wi83LQBeAAlNG3Ebl"
            },
            "mode_info": {
              "single": {
                "mode": "SIGN_MODE_DIRECT"
              }
            },
            "sequence": "0"
          }
        ],
        "fee": {
          "amount": [
            {
              "denom": "ubbn",
              "amount": "2000000"
            }
          ],
          "gas_limit": "200000",
          "payer": "",
          "granter": ""
        },
        "tip": null
      },
      "signatures": [
        "QMpALwCa+mHtKRv9Jg9RyHg/lOnFNa5i09tHgHgSuwh8JbNalYy4v2bTZ2PGDUh0JZCPUMeO487WacZofVhl9g=="
      ]
    }
  ]
}`
