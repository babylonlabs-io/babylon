package types

import (
	"context"
	"errors"

	txformat "github.com/babylonlabs-io/babylon/btctxformatter"
	bbn "github.com/babylonlabs-io/babylon/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type MockBTCLightClientKeeper struct {
	headers map[string]uint32
}

type MockCheckpointingKeeper struct {
	returnError bool
}

type MockIncentiveKeeper struct {
}

func NewMockBTCLightClientKeeper() *MockBTCLightClientKeeper {
	lc := MockBTCLightClientKeeper{
		headers: make(map[string]uint32),
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

func (m *MockCheckpointingKeeper) ReturnError() {
	m.returnError = true
}

func (m *MockCheckpointingKeeper) ReturnSuccess() {
	m.returnError = false
}

func (m *MockBTCLightClientKeeper) SetDepth(header *bbn.BTCHeaderHashBytes, dd uint32) {
	m.headers[header.String()] = dd
}

func (m *MockBTCLightClientKeeper) DeleteHeader(header *bbn.BTCHeaderHashBytes) {
	delete(m.headers, header.String())
}

func (m MockBTCLightClientKeeper) BlockHeight(ctx context.Context, header *bbn.BTCHeaderHashBytes) (uint32, error) {
	// todo not used
	return uint32(10), nil
}

func (m MockBTCLightClientKeeper) MainChainDepth(ctx context.Context, headerBytes *bbn.BTCHeaderHashBytes) (uint32, error) {
	depth, ok := m.headers[headerBytes.String()]
	if ok {
		return depth, nil
	} else {
		return 0, errors.New("unknown header")
	}
}

func (m MockCheckpointingKeeper) VerifyCheckpoint(ctx context.Context, checkpoint txformat.RawBtcCheckpoint) error {
	if m.returnError {
		return errors.New("bad checkpoints")
	}

	return nil
}

// SetCheckpointSubmitted Informs checkpointing module that checkpoint was
// successfully submitted on btc chain.
func (m MockCheckpointingKeeper) SetCheckpointSubmitted(ctx context.Context, epoch uint64) {
}

// SetCheckpointConfirmed Informs checkpointing module that checkpoint was
// successfully submitted on btc chain, and it is at least K-deep on the main chain
func (m MockCheckpointingKeeper) SetCheckpointConfirmed(ctx context.Context, epoch uint64) {
}

// SetCheckpointFinalized Informs checkpointing module that checkpoint was
// successfully submitted on btc chain, and it is at least W-deep on the main chain
func (m MockCheckpointingKeeper) SetCheckpointFinalized(ctx context.Context, epoch uint64) {
}

// SetCheckpointForgotten Informs checkpointing module that was in submitted state
// lost all its checkpoints and is checkpoint empty
func (m MockCheckpointingKeeper) SetCheckpointForgotten(ctx context.Context, epoch uint64) {
}

func (m *MockIncentiveKeeper) RewardBTCTimestamping(ctx context.Context, epoch uint64, rewardDistInfo *RewardDistInfo) {
}

func (m *MockIncentiveKeeper) IndexRefundableMsg(ctx context.Context, msg sdk.Msg) {
}
