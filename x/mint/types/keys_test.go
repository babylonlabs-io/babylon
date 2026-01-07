package types_test

import (
	"testing"

	"github.com/babylonlabs-io/babylon/v4/testutil/store"
	"github.com/babylonlabs-io/babylon/v4/x/mint/types"
)

func TestNoKeyCollision(t *testing.T) {
	keys := map[string]interface{}{
		"MinterKey":     types.MinterKey,
		"GenesisTimeKey": types.GenesisTimeKey,
	}

	store.CheckKeyCollisions(t, keys)
}
