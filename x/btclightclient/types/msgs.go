package types

import (
	"encoding/hex"
	"fmt"
	"math/big"

	bbn "github.com/babylonlabs-io/babylon/v3/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = (*MsgInsertHeaders)(nil)

func NewMsgInsertHeaders(signer sdk.AccAddress, headersHex string) (*MsgInsertHeaders, error) {
	if len(headersHex) == 0 {
		return nil, fmt.Errorf("empty headers list")
	}

	decoded, err := hex.DecodeString(headersHex)

	if err != nil {
		return nil, err
	}

	if len(decoded)%bbn.BTCHeaderLen != 0 {
		return nil, fmt.Errorf("invalid length of encoded headers: %d", len(decoded))
	}
	numOfHeaders := len(decoded) / bbn.BTCHeaderLen
	headers := make([]bbn.BTCHeaderBytes, numOfHeaders)

	for i := 0; i < numOfHeaders; i++ {
		headerSlice := decoded[i*bbn.BTCHeaderLen : (i+1)*bbn.BTCHeaderLen]
		headerBytes, err := bbn.NewBTCHeaderBytesFromBytes(headerSlice)
		if err != nil {
			return nil, err
		}
		headers[i] = headerBytes
	}
	return &MsgInsertHeaders{Signer: signer.String(), Headers: headers}, nil
}

func (msg *MsgInsertHeaders) ValidateHeaders(powLimit *big.Int) error {
	// TODO: Limit number of headers in message?
	for _, header := range msg.Headers {
		err := bbn.ValidateBTCHeader(header.ToBlockHeader(), powLimit)
		if err != nil {
			return err
		}
	}

	return nil
}

func (msg *MsgInsertHeaders) ReporterAddress() sdk.AccAddress {
	sender, err := sdk.AccAddressFromBech32(msg.Signer)
	if err != nil {
		panic(err)
	}
	return sender
}

func (msg *MsgInsertHeaders) ValidateStateless() error {
	_, err := sdk.AccAddressFromBech32(msg.Signer)

	if err != nil {
		return err
	}

	if len(msg.Headers) == 0 {
		return fmt.Errorf("empty headers list")
	}

	return nil
}
