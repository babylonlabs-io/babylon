package cli

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestParseMultisigInfoJSON_UnmarshalsBase64Sigs(t *testing.T) {
	tmp := t.TempDir()
	jsonData := `{
  "staker_btc_pk_list": [
    "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9"
  ],
  "staker_quorum": 1,
  "delegator_slashing_sigs": [
    {
      "pk": "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9",
      "sig": "BOf5A3ZYqSr+tPJbrlM5493KgaNTSTgn0m8W2SMI5J4qJekiCGeKLfhpcNqRsDqK+IFaimBJizWNr1YLNHqlVw=="
    }
  ],
  "delegator_unbonding_slashing_sigs": [
    {
      "pk": "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9",
      "sig": "WDGq7te0S7dOXquUup1ClMSbzypgco2LTCAPUN0xPBurdFh5pa2VSnLEWpHDpR08et6pjYL4SB4OHgNnSm8/tw=="
    }
  ]
}`
	path := tmp + "/multisig.json"
	require.NoError(t, os.WriteFile(path, []byte(jsonData), 0o644))

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String(FlagMultisigInfoJSON, "", "")
	require.NoError(t, fs.Parse([]string{"--" + FlagMultisigInfoJSON, path}))

	info, err := parseMultisigInfoJSON(fs)
	require.NoError(t, err)
	require.NotNil(t, info)

	expSig1, err := base64.StdEncoding.DecodeString("BOf5A3ZYqSr+tPJbrlM5493KgaNTSTgn0m8W2SMI5J4qJekiCGeKLfhpcNqRsDqK+IFaimBJizWNr1YLNHqlVw==")
	require.NoError(t, err)
	expSig2, err := base64.StdEncoding.DecodeString("WDGq7te0S7dOXquUup1ClMSbzypgco2LTCAPUN0xPBurdFh5pa2VSnLEWpHDpR08et6pjYL4SB4OHgNnSm8/tw==")
	require.NoError(t, err)

	require.Len(t, info.StakerBtcPkList, 1)
	require.Equal(t, "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9", (&info.StakerBtcPkList[0]).MarshalHex())

	require.Len(t, info.DelegatorSlashingSigs, 1)
	require.Equal(t, expSig1, []byte(*info.DelegatorSlashingSigs[0].Sig))
	require.Equal(t, "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9", info.DelegatorSlashingSigs[0].Pk.MarshalHex())

	require.Len(t, info.DelegatorUnbondingSlashingSigs, 1)
	require.Equal(t, expSig2, []byte(*info.DelegatorUnbondingSlashingSigs[0].Sig))
	require.Equal(t, "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9", info.DelegatorUnbondingSlashingSigs[0].Pk.MarshalHex())
}
