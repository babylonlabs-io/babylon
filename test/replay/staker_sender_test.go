package replay

import (
	"bytes"
	"encoding/hex"
	bbn "github.com/babylonlabs-io/babylon/v3/types"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
	"time"
)

func TestUnbondDelegation(t *testing.T) {
	tmpGas := DefaultGasLimit
	DefaultGasLimit = uint64(10_000_000)
	defer func() {
		DefaultGasLimit = tmpGas
	}()

	t.Parallel()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	d := NewBabylonAppDriverTmpDir(r, t)
	d.GenerateNewBlockAssertExecutionSuccess()

	covSender := d.CreateCovenantSender()
	bbnFp := d.CreateNFinalityProviderAccounts(1)[0]
	numStakers := 1
	stakers := d.CreateNStakerAccounts(numStakers)
	d.GenerateNewBlockAssertExecutionSuccess()

	bbnFp.RegisterFinalityProvider("")
	d.GenerateNewBlockAssertExecutionSuccess()

	bbnFp.CommitRandomness()

	currentEpochNumber := d.GetEpoch().EpochNumber
	d.ProgressTillFirstBlockTheNextEpoch()
	d.FinializeCkptForEpoch(currentEpochNumber)

	d.GenerateNewBlockAssertExecutionSuccess()

	stk2 := stakers[0]
	// send one btc delegation just to have voting power in the fp
	fps := []*bbn.BIP340PubKey{bbnFp.BTCPublicKey()}

	d.SendAndVerifyNDelegations(t, stk2, covSender, fps, 1)

	d.ActivateVerifiedDelegations(1)
	d.GenerateNewBlockAssertExecutionSuccess()

	// fp is activated
	activationHeight := d.GetActivationHeight(d.t)
	require.Greater(d.t, activationHeight, uint64(0))

	totalActiveDels := len(d.GetActiveBTCDelegations(d.t))
	require.Greater(t, totalActiveDels, 0)

	activeDelegations := d.GetActiveBTCDelegations(d.t)
	require.Len(t, activeDelegations, 1)
	activation := activeDelegations[0]

	stakingTx := &wire.MsgTx{}
	txBytes, err := hex.DecodeString(activation.StakingTxHex)
	require.NoError(t, err)
	err = stakingTx.Deserialize(bytes.NewReader(txBytes))
	require.NoError(t, err)

	stakingTxHash := stakingTx.TxHash()

	stk2.UnbondDelegation(&stakingTxHash, stakingTx, covSender)

	d.GenerateNewBlockAssertExecutionSuccess()

	unbondedDels := d.GetUnbondedBTCDelegations(d.t)
	require.Greater(t, len(unbondedDels), 0)

	totalActiveDelsFinal := len(d.GetActiveBTCDelegations(d.t))
	require.Equal(t, totalActiveDelsFinal, 0)
}
