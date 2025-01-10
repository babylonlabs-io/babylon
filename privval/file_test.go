package privval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/babylonlabs-io/babylon/crypto/erc2335"
	cmtcfg "github.com/cometbft/cometbft/config"
	cmtos "github.com/cometbft/cometbft/libs/os"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/test-go/testify/assert"
)

func TestSaveDelegatorAddressToFile(t *testing.T) {

	tempDir := t.TempDir()
	defer os.RemoveAll(tempDir)

	ccfg := cmtcfg.DefaultConfig()
	ccfg.SetRoot(tempDir)
	bcfg := DefaultBlsConfig()
	bcfg.SetRoot(tempDir)

	pvKeyFile := filepath.Join(tempDir, ccfg.PrivValidatorKeyFile())
	pvStateFile := filepath.Join(tempDir, ccfg.PrivValidatorStateFile())
	blsKeyFile := filepath.Join(tempDir, bcfg.BlsKeyFile())
	blsPasswordFile := filepath.Join(tempDir, bcfg.BlsPasswordFile())

	err := cmtos.EnsureDir(filepath.Dir(pvKeyFile), 0777)
	assert.Nil(t, err)
	err = cmtos.EnsureDir(filepath.Dir(pvStateFile), 0777)
	assert.Nil(t, err)
	err = cmtos.EnsureDir(filepath.Dir(blsKeyFile), 0777)
	assert.Nil(t, err)
	err = cmtos.EnsureDir(filepath.Dir(blsPasswordFile), 0777)
	assert.Nil(t, err)

	pv := GenWrappedFilePV(
		pvKeyFile,
		pvStateFile,
		blsKeyFile,
		blsPasswordFile,
	)

	pv.Save("test")

	t.Run("save delegator address", func(t *testing.T) {
		delegatorAddress := pv.Key.CometPVKey.PubKey.Address()
		t.Log("delegatorAddress: ", delegatorAddress)

		// addr, err := sdk.AccAddressFromBech32(delegatorAddress.String())
		// assert.NoError(t, err)
		// t.Log("sdk.AccAddressFromBech32(delegatorAddress.String()): ", addr)

		pv.SetAccAddress(types.AccAddress(delegatorAddress))

		var keystore erc2335.Erc2335KeyStore
		keyJSONBytes, err := os.ReadFile(blsKeyFile)
		assert.NoError(t, err)
		err = json.Unmarshal(keyJSONBytes, &keystore)
		assert.NoError(t, err)

		// t.Log(keystore)
		t.Log(keystore.Description)
	})

	t.Run("load delegator address", func(t *testing.T) {
		delegatorAddress := ReadDelegatorAddressFromFile(blsKeyFile)
		assert.Equal(t, delegatorAddress, pv.Key.DelegatorAddress)
	})
}
