package types

import "errors"

var (
	ErrUnmarshal            = errors.New("unmarshal error")
	ErrInvalidAmount        = errors.New("invalid amount")
	ErrOutputNotFound       = errors.New("output not found")
	ErrMultipleOutputsMatch = errors.New("multiple outputs match the given PkScript and Value")
)
