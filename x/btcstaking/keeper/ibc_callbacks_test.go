package keeper_test

import (
	"math/rand"
	"testing"
	"time"

	"cosmossdk.io/math"
	appparams "github.com/babylonlabs-io/babylon/v4/app/params"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	"github.com/stretchr/testify/require"

	"github.com/babylonlabs-io/babylon/v4/x/btcstaking/keeper"
)

// mockPacket implements ibcexported.PacketI for testing purposes
type mockPacket struct {
	sourcePort       string
	sourceChannel    string
	destPort         string
	destChannel      string
	data             []byte
	sequence         uint64
	timeoutHeight    clienttypes.Height
	timeoutTimestamp uint64
}

// NewMockPacket creates a new mock packet with the specified source and destination ports/channels
func NewMockPacket(sourcePort, sourceChannel, destPort, destChannel string) ibcexported.PacketI {
	return &mockPacket{
		sourcePort:    sourcePort,
		sourceChannel: sourceChannel,
		destPort:      destPort,
		destChannel:   destChannel,
		data:          []byte("test-data"),
		sequence:      1,
	}
}

// Implement ibcexported.PacketI interface
func (p *mockPacket) GetSourcePort() string                { return p.sourcePort }
func (p *mockPacket) GetSourceChannel() string             { return p.sourceChannel }
func (p *mockPacket) GetDestPort() string                  { return p.destPort }
func (p *mockPacket) GetDestChannel() string               { return p.destChannel }
func (p *mockPacket) GetData() []byte                      { return p.data }
func (p *mockPacket) GetSequence() uint64                  { return p.sequence }
func (p *mockPacket) GetTimeoutHeight() ibcexported.Height { return p.timeoutHeight }
func (p *mockPacket) GetTimeoutTimestamp() uint64          { return p.timeoutTimestamp }
func (p *mockPacket) ValidateBasic() error                 { return nil }

func TestBabylonRepresentationIcs20TransferCoin(t *testing.T) {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	packet := NewMockPacket("transfer", "channel-0", "transfer", "channel-0")

	transferAmt := r.Int63n(2_000000) + 1_000000
	transferInt := math.NewInt(transferAmt)

	// test any coin coming from BSN to babylon
	bsnDenom := "factory/bbn18ksee06lpvhrv6vxrtcsuju6sdkxzhww20w6fr/ee6f3aa622b84287391b"
	bsnCoin, err := keeper.BabylonRepresentationIcs20TransferCoin(packet, types.Token{
		Denom:  types.ExtractDenomFromPath(bsnDenom),
		Amount: transferInt.String(),
	})
	require.NoError(t, err)
	expectedBsnDenomAsIbcOnBabylon := "ibc/10AC285CC67C4B5F8D6F5AD86102143C087FEE2AFBD83AB87E2AEB494E2951CB"
	require.Equal(t, bsnCoin.String(), sdk.NewCoin(expectedBsnDenomAsIbcOnBabylon, transferInt).String())

	// test native ubbn that was first on babylon to BSN and then BSN
	// is returning it to babylon
	ubbnDenomInBsn := "transfer/channel-0/ubbn"
	ubbnCoin, err := keeper.BabylonRepresentationIcs20TransferCoin(packet, types.Token{
		Denom:  types.ExtractDenomFromPath(ubbnDenomInBsn),
		Amount: transferInt.String(),
	})
	require.NoError(t, err)
	require.Equal(t, ubbnCoin.String(), sdk.NewCoin(appparams.DefaultBondDenom, transferInt).String())
}
