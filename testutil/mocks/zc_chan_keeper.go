package mocks

import (
	context "context"
	reflect "reflect"

	"github.com/cosmos/cosmos-sdk/types"
	exported "github.com/cosmos/ibc-go/v10/modules/core/exported"

	gomock "github.com/golang/mock/gomock"
)

// MockZoneConciergeChannelKeeper is a mock of ZoneConciergeChannelKeeper interface.
type MockZoneConciergeChannelKeeper struct {
	ctrl     *gomock.Controller
	recorder *MockZoneConciergeChannelKeeperMockRecorder
}

// MockZoneConciergeChannelKeeperMockRecorder is the mock recorder for MockZoneConciergeChannelKeeper.
type MockZoneConciergeChannelKeeperMockRecorder struct {
	mock *MockZoneConciergeChannelKeeper
}

// NewMockZoneConciergeChannelKeeper creates a new mock instance.
func NewMockZoneConciergeChannelKeeper(ctrl *gomock.Controller) *MockZoneConciergeChannelKeeper {
	mock := &MockZoneConciergeChannelKeeper{ctrl: ctrl}
	mock.recorder = &MockZoneConciergeChannelKeeperMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockZoneConciergeChannelKeeper) EXPECT() *MockZoneConciergeChannelKeeperMockRecorder {
	return m.recorder
}

// ConsumerHasIBCChannelOpen mocks base method.
func (m *MockZoneConciergeChannelKeeper) ConsumerHasIBCChannelOpen(ctx context.Context, consumerID, channelID string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ConsumerHasIBCChannelOpen", ctx, consumerID, channelID)
	ret0, _ := ret[0].(bool)
	return ret0
}

// ConsumerHasIBCChannelOpen indicates an expected call of ConsumerHasIBCChannelOpen.
func (mr *MockZoneConciergeChannelKeeperMockRecorder) ConsumerHasIBCChannelOpen(ctx, consumerID, channelID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ConsumerHasIBCChannelOpen", reflect.TypeOf((*MockZoneConciergeChannelKeeper)(nil).ConsumerHasIBCChannelOpen), ctx, consumerID, channelID)
}

// GetChannelClientState mocks base method.
func (m *MockZoneConciergeChannelKeeper) GetChannelClientState(ctx types.Context, portID, channelID string) (string, exported.ClientState, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetChannelClientState", ctx, portID, channelID)
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(exported.ClientState)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// GetChannelClientState indicates an expected call of GetChannelClientState.
func (mr *MockZoneConciergeChannelKeeperMockRecorder) GetChannelClientState(ctx, portID, channelID interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetChannelClientState", reflect.TypeOf((*MockZoneConciergeChannelKeeper)(nil).GetChannelClientState), ctx, portID, channelID)
}
