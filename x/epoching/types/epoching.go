package types

import (
	"context"
	"errors"
	"fmt"
	"time"

	errorsmod "cosmossdk.io/errors"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/cometbft/cometbft/crypto/tmhash"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewEpoch constructs a new Epoch object
// The relationship between block and epoch is as follows, assuming epoch interval of 5:
// 0 | 1 2 3 4 5 | 6 7 8 9 10 |
// 0 |     1     |     2      |
func NewEpoch(epochNumber uint64, epochInterval uint64, firstBlockHeight uint64, lastBlockTime *time.Time) Epoch {
	return Epoch{
		EpochNumber:          epochNumber,
		CurrentEpochInterval: epochInterval,
		FirstBlockHeight:     firstBlockHeight,
		LastBlockTime:        lastBlockTime,
		// NOTE: SealerHeader will be set in the next epoch
	}
}

func (e Epoch) GetLastBlockHeight() uint64 {
	if e.EpochNumber == 0 {
		return 0
	}
	return e.FirstBlockHeight + e.CurrentEpochInterval - 1
}

func (e Epoch) GetSealerBlockHeight() uint64 {
	return e.GetLastBlockHeight() + 1
}

func (e Epoch) GetSecondBlockHeight() uint64 {
	if e.EpochNumber == 0 {
		panic("should not be called when epoch number is zero")
	}
	return e.FirstBlockHeight + 1
}

func (e Epoch) IsLastBlock(ctx context.Context) bool {
	return e.GetLastBlockHeight() == uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
}

func (e Epoch) IsLastBlockByHeight(height int64) bool {
	return e.GetLastBlockHeight() == uint64(height)
}

func (e Epoch) IsFirstBlock(ctx context.Context) bool {
	return e.FirstBlockHeight == uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
}

func (e Epoch) IsSecondBlock(ctx context.Context) bool {
	if e.EpochNumber == 0 {
		return false
	}
	return e.GetSecondBlockHeight() == uint64(sdk.UnwrapSDKContext(ctx).HeaderInfo().Height)
}

func (e Epoch) IsVoteExtensionProposal(ctx context.Context) bool {
	if e.EpochNumber == 0 {
		return false
	}
	return e.IsFirstBlockOfNextEpoch(ctx)
}

// IsFirstBlockOfNextEpoch checks whether the current block is the first block of
// the next epoch
// CONTRACT: IsFirstBlockOfNextEpoch can only be called by the epoching module
// once upon the first block of a new epoch
// other modules should use IsFirstBlock instead.
func (e Epoch) IsFirstBlockOfNextEpoch(ctx context.Context) bool {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	if e.EpochNumber == 0 {
		return sdkCtx.HeaderInfo().Height == 1
	} else {
		height := uint64(sdkCtx.HeaderInfo().Height)
		return e.FirstBlockHeight+e.CurrentEpochInterval == height
	}
}

// WithinBoundary checks whether the given height is within this epoch or not
func (e Epoch) WithinBoundary(height uint64) bool {
	if height < e.FirstBlockHeight || height > e.GetLastBlockHeight() {
		return false
	} else {
		return true
	}
}

// ValidateBasic does sanity checks on Epoch
func (e Epoch) ValidateBasic() error {
	if e.CurrentEpochInterval < 2 {
		return ErrInvalidEpoch.Wrapf("CurrentEpochInterval (%d) < 2", e.CurrentEpochInterval)
	}
	return nil
}

// NewQueuedMessage creates a new QueuedMessage from a wrapped msg
// i.e., wrapped -> unwrapped -> QueuedMessage
func NewQueuedMessage(blockHeight uint64, blockTime time.Time, txid []byte, msg sdk.Msg) (QueuedMessage, error) {
	// marshal the actual msg (MsgDelegate, MsgBeginRedelegate, MsgUndelegate, MsgCancelUnbondingDelegation) inside isQueuedMessage_Msg
	var qmsg isQueuedMessage_Msg
	var msgBytes []byte
	var err error
	switch msgWithType := msg.(type) {
	case *MsgWrappedDelegate:
		if msgBytes, err = msgWithType.Msg.Marshal(); err != nil {
			return QueuedMessage{}, err
		}
		qmsg = &QueuedMessage_MsgDelegate{
			MsgDelegate: msgWithType.Msg,
		}
	case *MsgWrappedBeginRedelegate:
		if msgBytes, err = msgWithType.Msg.Marshal(); err != nil {
			return QueuedMessage{}, err
		}
		qmsg = &QueuedMessage_MsgBeginRedelegate{
			MsgBeginRedelegate: msgWithType.Msg,
		}
	case *MsgWrappedUndelegate:
		if msgBytes, err = msgWithType.Msg.Marshal(); err != nil {
			return QueuedMessage{}, err
		}
		qmsg = &QueuedMessage_MsgUndelegate{
			MsgUndelegate: msgWithType.Msg,
		}
	case *MsgWrappedCancelUnbondingDelegation:
		if msgBytes, err = msgWithType.Msg.Marshal(); err != nil {
			return QueuedMessage{}, err
		}
		qmsg = &QueuedMessage_MsgCancelUnbondingDelegation{
			MsgCancelUnbondingDelegation: msgWithType.Msg,
		}
	case *stakingtypes.MsgCreateValidator:
		if msgBytes, err = msgWithType.Marshal(); err != nil {
			return QueuedMessage{}, err
		}
		qmsg = &QueuedMessage_MsgCreateValidator{
			MsgCreateValidator: msgWithType,
		}
	case *stakingtypes.MsgEditValidator:
		if msgBytes, err = msgWithType.Marshal(); err != nil {
			return QueuedMessage{}, err
		}
		qmsg = &QueuedMessage_MsgEditValidator{
			MsgEditValidator: msgWithType,
		}
	case *stakingtypes.MsgUpdateParams:
		if msgBytes, err = msgWithType.Marshal(); err != nil {
			return QueuedMessage{}, err
		}
		qmsg = &QueuedMessage_MsgUpdateParams{
			MsgUpdateParams: msgWithType,
		}
	default:
		return QueuedMessage{}, ErrUnwrappedMsgType
	}

	queuedMsg := QueuedMessage{
		TxId:        txid,
		MsgId:       tmhash.Sum(msgBytes),
		BlockHeight: blockHeight,
		BlockTime:   &blockTime,
		Msg:         qmsg,
	}
	return queuedMsg, nil
}

// UnpackInterfaces implements UnpackInterfacesMessage.UnpackInterfaces
func (qm QueuedMessage) UnpackInterfaces(unpacker codectypes.AnyUnpacker) error {
	var pubKey cryptotypes.PubKey
	msgWithType, ok := qm.UnwrapToSdkMsg().(*stakingtypes.MsgCreateValidator)
	if !ok {
		return nil
	}
	return unpacker.UnpackAny(msgWithType.Pubkey, &pubKey)
}

func (qm *QueuedMessage) UnwrapToSdkMsg() sdk.Msg {
	var unwrappedMsgWithType sdk.Msg
	switch unwrappedMsg := qm.Msg.(type) {
	case *QueuedMessage_MsgCreateValidator:
		unwrappedMsgWithType = unwrappedMsg.MsgCreateValidator
	case *QueuedMessage_MsgDelegate:
		unwrappedMsgWithType = unwrappedMsg.MsgDelegate
	case *QueuedMessage_MsgUndelegate:
		unwrappedMsgWithType = unwrappedMsg.MsgUndelegate
	case *QueuedMessage_MsgBeginRedelegate:
		unwrappedMsgWithType = unwrappedMsg.MsgBeginRedelegate
	case *QueuedMessage_MsgCancelUnbondingDelegation:
		unwrappedMsgWithType = unwrappedMsg.MsgCancelUnbondingDelegation
	case *QueuedMessage_MsgEditValidator:
		unwrappedMsgWithType = unwrappedMsg.MsgEditValidator
	case *QueuedMessage_MsgUpdateParams:
		unwrappedMsgWithType = unwrappedMsg.MsgUpdateParams
	default:
		panic(errorsmod.Wrap(ErrInvalidQueuedMessageType, qm.String()))
	}
	return unwrappedMsgWithType
}

func (e Validator) Validate() error {
	if e.Power < 0 {
		return fmt.Errorf("validator power cannot be negative: got %d", e.Power)
	}

	valAddrStr := sdk.ValAddress(e.Addr).String()
	_, err := sdk.ValAddressFromBech32(valAddrStr)
	return err
}

func (vl ValidatorLifecycle) Validate() error {
	if len(vl.ValLife) == 0 {
		return errors.New("validator lyfecycle is empty")
	}

	for _, vsu := range vl.ValLife {
		if err := vsu.Validate(); err != nil {
			return err
		}
	}

	_, err := sdk.ValAddressFromBech32(vl.ValAddr)
	return err
}

func (v ValStateUpdate) Validate() error {
	return ValidateBondState(v.State)
}

func ValidateBondState(bs BondState) error {
	switch bs {
	case BondState_CREATED:
		return nil
	case BondState_BONDED:
		return nil
	case BondState_UNBONDING:
		return nil
	case BondState_UNBONDED:
		return nil
	case BondState_REMOVED:
		return nil
	default:
		return fmt.Errorf("invalid bond state: %d", bs)
	}
}

func (dl DelegationLifecycle) Validate() error {
	if len(dl.DelLife) == 0 {
		return errors.New("delegation lyfecycle is empty")
	}
	_, err := sdk.AccAddressFromBech32(dl.DelAddr)

	for _, vsu := range dl.DelLife {
		if err := vsu.Validate(); err != nil {
			return err
		}
	}

	return err
}

func (d DelegationStateUpdate) Validate() error {
	if err := ValidateBondState(d.State); err != nil {
		return err
	}

	_, err := sdk.ValAddressFromBech32(d.ValAddr)
	return err
}
