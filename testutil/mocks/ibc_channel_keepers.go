package mocks

import (
	sdk "github.com/cosmos/cosmos-sdk/types"

	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	"github.com/cosmos/ibc-go/v8/modules/core/exported"

	"github.com/test-go/testify/mock"
)

var _ transfertypes.ChannelKeeper = &MockChannelKeeper{}

type MockChannelKeeper struct {
	mock.Mock
}

//nolint:revive // allow unused parameters to indicate expected signature
func (b *MockChannelKeeper) GetChannel(ctx sdk.Context, srcPort, srcChan string) (channel channeltypes.Channel, found bool) {
	args := b.Called(mock.Anything, mock.Anything, mock.Anything)
	return args.Get(0).(channeltypes.Channel), true
}

//nolint:revive // allow unused parameters to indicate expected signature
func (b *MockChannelKeeper) GetNextSequenceSend(ctx sdk.Context, portID, channelID string) (uint64, bool) {
	_ = b.Called(mock.Anything, mock.Anything, mock.Anything)
	return 1, true
}

//nolint:revive // allow unused parameters to indicate expected signature
func (b *MockChannelKeeper) GetAllChannelsWithPortPrefix(ctx sdk.Context, portPrefix string) []channeltypes.IdentifiedChannel {
	return []channeltypes.IdentifiedChannel{}
}

func (b *MockChannelKeeper) GetChannelClientState(ctx sdk.Context, portID string, channelID string) (string, exported.ClientState, error) {
	args := b.Called(mock.Anything, mock.Anything, mock.Anything)
	return "", args.Get(0).(exported.ClientState), nil
}
