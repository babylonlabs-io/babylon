package replay

import (
	"math/rand"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/testutil/testdata"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/stretchr/testify/require"
)

func TestReplayBlocks(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)

	for i := 0; i < 100; i++ {
		driver.GenerateNewBlock()
	}

	replayer := NewBlockReplayer(t, replayerTempDir)
	replayer.ReplayBlocks(t, driver.GetFinalizedBlocks())

	// after replay we should have the same apphash
	require.Equal(t, driver.GetLastState().LastBlockHeight, replayer.LastState.LastBlockHeight)
	require.Equal(t, driver.GetLastState().AppHash, replayer.LastState.AppHash)
}

func TestSendingTxFromDriverAccount(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	driverTempDir := t.TempDir()
	replayerTempDir := t.TempDir()
	driver := NewBabylonAppDriver(r, t, driverTempDir, replayerTempDir)

	// go over epoch boundary
	for i := 0; i < 1+epochLength; i++ {
		driver.GenerateNewBlock()
	}

	_, _, addr1 := testdata.KeyTestPubAddr()
	toAddr := addr1.String()

	transferMsg := &banktypes.MsgSend{
		FromAddress: driver.GetDriverAccountAddress().String(),
		ToAddress:   toAddr,
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("ubbn", 10000)),
	}

	driver.SendTxWithMsgsFromDriverAccount(t, transferMsg)
	driver.SendTxWithMsgsFromDriverAccount(t, transferMsg)
	driver.SendTxWithMsgsFromDriverAccount(t, transferMsg)
	driver.SendTxWithMsgsFromDriverAccount(t, transferMsg)

	// check that replayer has the same state as driver, as we replayed all blocks
	replayer := NewBlockReplayer(t, replayerTempDir)
	replayer.ReplayBlocks(t, driver.GetFinalizedBlocks())

	// after replay we should have the same apphash
	require.Equal(t, driver.GetLastState().LastBlockHeight, replayer.LastState.LastBlockHeight)
	require.Equal(t, driver.GetLastState().AppHash, replayer.LastState.AppHash)
}
