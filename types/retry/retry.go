package retry

import (
	"crypto/rand"
	"errors"
	"math/big"
	"time"

	btcctypes "github.com/babylonlabs-io/babylon/x/btccheckpoint/types"
	btclctypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
	checkpointingtypes "github.com/babylonlabs-io/babylon/x/checkpointing/types"
)

// unrecoverableErrors is a list of errors which are unsafe and should not be retried.
var unrecoverableErrors = []error{
	btclctypes.ErrHeaderParentDoesNotExist,
	btclctypes.ErrChainWithNotEnoughWork,
	btclctypes.ErrInvalidHeader,
	btcctypes.ErrProvidedHeaderDoesNotHaveAncestor,
	btcctypes.ErrInvalidHeader,
	btcctypes.ErrNoCheckpointsForPreviousEpoch,
	btcctypes.ErrInvalidCheckpointProof,
	checkpointingtypes.ErrBlsPrivKeyDoesNotExist,
	// TODO Add more errors here
}

// expectedErrors is a list of errors which can safely be ignored and should not be retried.
var expectedErrors = []error{
	btcctypes.ErrDuplicatedSubmission,
	btcctypes.ErrInvalidHeader,
	// TODO Add more errors here
}

func isUnrecoverableErr(err error) bool {
	for _, e := range unrecoverableErrors {
		if errors.Is(err, e) {
			return true
		}
	}

	return false
}

func isExpectedErr(err error) bool {
	for _, e := range expectedErrors {
		if errors.Is(err, e) {
			return true
		}
	}

	return false
}

// Do executes a func with retry
// TODO: Remove this function, and make our programs to depend on retires based
// on some standard retry library
func Do(sleep time.Duration, maxSleepTime time.Duration, retryableFunc func() error) error {
	if err := retryableFunc(); err != nil {
		if isUnrecoverableErr(err) {
			logger.Error("Skip retry, error unrecoverable", "err", err)
			return err
		}

		if isExpectedErr(err) {
			logger.Error("Skip retry, error expected", "err", err)
			return nil
		}

		// Add some randomness to prevent thrashing
		// TODO: This duration should be passed by the caller
		jitter, randomnessErr := randDuration(int64(sleep))
		if randomnessErr != nil {
			return randomnessErr
		}
		sleep = sleep + jitter/2

		if sleep > maxSleepTime {
			logger.Info("retry timed out")
			return err
		}

		logger.Info("starting exponential backoff", "sleep", sleep, "err", err)
		time.Sleep(sleep)

		return Do(2*sleep, maxSleepTime, retryableFunc)
	}
	return nil
}

func randDuration(maxNumber int64) (dur time.Duration, err error) {
	randNumber, err := rand.Int(rand.Reader, big.NewInt(maxNumber))
	if err != nil {
		return dur, err
	}
	return time.Duration(randNumber.Int64()), nil
}
