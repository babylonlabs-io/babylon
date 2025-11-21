package cli

import (
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
)

func TestParseMultisigInfoJSON_UnmarshalsBIP340Types(t *testing.T) {
	tmp := t.TempDir()

	jsonData := `{
  "staker_btc_pk_list": [
    "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9"
  ],
  "staker_quorum": 2,
  "delegator_slashing_sigs": [
    {
      "pk": "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9",
      "sig": "04e7f9037658a92afeb4f25bae5339e3ddca81a353493827d26f16d92308e49e2a25e92208678a2df86970da91b03a8af8815a8a60498b358daf560b347aa557"
    }
  ],
  "delegator_unbonding_slashing_sigs": [
    {
      "pk": "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9",
      "sig": "5831aaeed7b44bb74e5eab94ba9d4294c49bcf2a60728d8b4c200f50dd313c1bab745879a5ad954a72c45a91c3a51d3c7adea98d82f8481e0e1e03674a6f3fb7"
    }
  ]
}`
	path := tmp + "/multisig.json"
	err := os.WriteFile(path, []byte(jsonData), 0o644)
	require.NoError(t, err)

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String(FlagMultisigInfoJSON, "", "")
	require.NoError(t, fs.Parse([]string{"--" + FlagMultisigInfoJSON, path}))

	info, err := parseMultisigInfoJSON(fs)
	require.NoError(t, err)
	require.NotNil(t, info)

	require.Len(t, info.StakerBtcPkList, 1)
	require.Equal(t, "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9", (&info.StakerBtcPkList[0]).MarshalHex())

	require.Equal(t, info.StakerQuorum, uint32(2))

	require.Len(t, info.DelegatorSlashingSigs, 1)
	require.Equal(t, "04e7f9037658a92afeb4f25bae5339e3ddca81a353493827d26f16d92308e49e2a25e92208678a2df86970da91b03a8af8815a8a60498b358daf560b347aa557", info.DelegatorSlashingSigs[0].Sig.ToHexStr())
	require.Equal(t, "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9", info.DelegatorSlashingSigs[0].Pk.MarshalHex())

	require.Len(t, info.DelegatorUnbondingSlashingSigs, 1)
	require.Equal(t, "5831aaeed7b44bb74e5eab94ba9d4294c49bcf2a60728d8b4c200f50dd313c1bab745879a5ad954a72c45a91c3a51d3c7adea98d82f8481e0e1e03674a6f3fb7", info.DelegatorUnbondingSlashingSigs[0].Sig.ToHexStr())
	require.Equal(t, "f9308a019258c31049344f85f89d5229b531c845836f99b08601f113bce036f9", info.DelegatorUnbondingSlashingSigs[0].Pk.MarshalHex())
}
