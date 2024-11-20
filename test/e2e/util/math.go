package util

import "golang.org/x/exp/constraints"

func Abs[T constraints.Integer](x T) T {
	if x < 0 {
		return -x
	}
	return x
}
