package types

import "errors"

var (
	ErrUnmarshal     = errors.New("unmarshal error")
	ErrInvalidAmount = errors.New("invalid amount")
)
