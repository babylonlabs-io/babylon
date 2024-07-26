package types

import (
	"context"
	"errors"

	txformat "github.com/babylonlabs-io/babylon/btctxformatter"
	bbn "github.com/babylonlabs-io/babylon/types"
)

type MockBTCLightClientKeeper struct {
	headers map[string]uint64
}

type MockCheckpointingKeeper struct {
	returnError bool
}

type MockIncentiveKeeper struct {
}

func NewMockBTCLightClientKeeper() *MockBTCLightClientKeeper {
	lc := MockBTCLightClientKeeper{
		headers: make(map[string]uint64),
	}
	return &lc
}

func NewMockCheckpointingKeeper() *MockCheckpointingKeeper {
	mc := MockCheckpointingKeeper{
		returnError: false,
	}
	return &mc
}

func NewMockIncentiveKeeper() *MockIncentiveKeeper {
	return &MockIncentiveKeeper{}
}

func (mc *MockCheckpointingKeeper) ReturnError() {
	mc.returnError = true
}

func (mc *MockCheckpointingKeeper) ReturnSuccess() {
	mc.returnError = false
}

func (mc *MockBTCLightClientKeeper) SetDepth(header *bbn.BTCHeaderHashBytes, dd uint64) {
	mc.headers[header.String()] = dd
}

func (mc *MockBTCLightClientKeeper) DeleteHeader(header *bbn.BTCHeaderHashBytes) {
	delete(mc.headers, header.String())
}

func (mb MockBTCLightClientKeeper) BlockHeight(ctx context.Context, header *bbn.BTCHeaderHashBytes) (uint64, error) {
	// todo not used
	return uint64(10), nil
}

func (ck MockBTCLightClientKeeper) MainChainDepth(ctx context.Context, headerBytes *bbn.BTCHeaderHashBytes) (uint64, error) {
	depth, ok := ck.headers[headerBytes.String()]
	if ok {
		return depth, nil
	} else {
		return 0, errors.New("unknown header")
	}
}

func (ck MockCheckpointingKeeper) VerifyCheckpoint(ctx context.Context, checkpoint txformat.RawBtcCheckpoint) error {
	if ck.returnError {
		return errors.New("bad checkpoints")
	}

	return nil
}

// SetCheckpointSubmitted Informs checkpointing module that checkpoint was
// successfully submitted on btc chain.
func (ck MockCheckpointingKeeper) SetCheckpointSubmitted(ctx context.Context, epoch uint64) {
}

// SetCheckpointConfirmed Informs checkpointing module that checkpoint was
// successfully submitted on btc chain, and it is at least K-deep on the main chain
func (ck MockCheckpointingKeeper) SetCheckpointConfirmed(ctx context.Context, epoch uint64) {
}

// SetCheckpointFinalized Informs checkpointing module that checkpoint was
// successfully submitted on btc chain, and it is at least W-deep on the main chain
func (ck MockCheckpointingKeeper) SetCheckpointFinalized(ctx context.Context, epoch uint64) {
}

// SetCheckpointForgotten Informs checkpointing module that was in submitted state
// lost all its checkpoints and is checkpoint empty
func (ck MockCheckpointingKeeper) SetCheckpointForgotten(ctx context.Context, epoch uint64) {
}

func (ik *MockIncentiveKeeper) RewardBTCTimestamping(ctx context.Context, epoch uint64, rewardDistInfo *RewardDistInfo) {
}
