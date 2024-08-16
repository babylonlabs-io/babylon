package retry

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUnrecoverableError(t *testing.T) {
	err := Do(1*time.Second, 1*time.Minute, func() error {
		return unrecoverableErrors[0]
	})
	require.Error(t, err)
}

func TestExpectedError(t *testing.T) {
	err := Do(1*time.Second, 1*time.Minute, func() error {
		return expectedErrors[0]
	})
	require.NoError(t, err)
}

func TestDoNotShadowAnError(t *testing.T) {
	var expectedError = errors.New("expected error")

	err := Do(1*time.Second, 1*time.Second, func() error {
		return expectedError
	})
	require.Error(t, err)
	require.ErrorIs(t, err, expectedError)
}
