package types_test

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/babylonlabs-io/babylon/v4/testutil/datagen"
	epochingtypes "github.com/babylonlabs-io/babylon/v4/x/epoching/types"
	"github.com/stretchr/testify/require"
)

func TestValidatorSet_FindValidatorWithIndex(t *testing.T) {
	valSet1 := datagen.GenRandomValSet(10)
	valSet2 := datagen.GenRandomValSet(1)
	for i := 0; i < len(valSet1); i++ {
		val, index, err := valSet1.FindValidatorWithIndex(valSet1[i].Addr)
		require.NoError(t, err)
		require.Equal(t, val, &valSet1[i])
		require.Equal(t, index, i)
	}
	val, index, err := valSet1.FindValidatorWithIndex(valSet2[0].Addr)
	require.Error(t, err)
	require.Nil(t, val)
	require.Equal(t, 0, index)
}

func TestNewSortedValidatorSetOrdersAscending(t *testing.T) {
	raw := []epochingtypes.Validator{
		{Addr: sdk.ValAddress("c")},
		{Addr: sdk.ValAddress("a")},
		{Addr: sdk.ValAddress("b")},
	}

	sorted := epochingtypes.NewSortedValidatorSet(raw)

	got := make([]sdk.ValAddress, len(sorted))
	for i := range sorted {
		got[i] = sorted[i].Addr
	}

	require.Equal(t, []sdk.ValAddress{
		sdk.ValAddress("a"),
		sdk.ValAddress("b"),
		sdk.ValAddress("c"),
	}, got)
}

func TestValidatorSetBinarySearchUsesFullAddress(t *testing.T) {
	raw := []epochingtypes.Validator{
		{Addr: sdk.ValAddress("a")},
		{Addr: sdk.ValAddress("b")},
		{Addr: sdk.ValAddress("c")},
	}

	valSet := epochingtypes.NewSortedValidatorSet(raw)

	target := sdk.ValAddress("b")
	val, idx, err := valSet.FindValidatorWithIndex(target)
	require.NoError(t, err)
	require.Equal(t, target, sdk.ValAddress(val.Addr))
	require.Equal(t, 1, idx)

	missing := sdk.ValAddress("d")
	val, idx, err = valSet.FindValidatorWithIndex(missing)
	require.Error(t, err)
	require.Nil(t, val)
	require.Zero(t, idx)
}

func TestValidatorSetBinarySearchHandlesBigEndianPrefix(t *testing.T) {
	addr1 := makeAddr(0x10)
	addr2 := makeAddr(0x20)
	addr3 := makeAddr(0x30)

	raw := []epochingtypes.Validator{
		{Addr: addr3},
		{Addr: addr1},
		{Addr: addr2},
	}

	valSet := epochingtypes.NewSortedValidatorSet(raw)
	got := make([]sdk.ValAddress, len(valSet))
	for i := range valSet {
		got[i] = valSet[i].Addr
	}

	require.Equal(t, []sdk.ValAddress{
		addr1,
		addr2,
		addr3,
	}, got)

	val, idx, err := valSet.FindValidatorWithIndex(addr2)
	require.NoError(t, err)
	require.Equal(t, addr2, sdk.ValAddress(val.Addr))
	require.Equal(t, 1, idx)

	missing := makeAddr(0x40)
	val, idx, err = valSet.FindValidatorWithIndex(missing)
	require.Error(t, err)
	require.Nil(t, val)
	require.Zero(t, idx)
}

func makeAddr(marker byte) sdk.ValAddress {
	basePrefix := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF, 0x11, 0x22}

	addr := make([]byte, 20)
	copy(addr, basePrefix) // first 8 bytes identical
	addr[8] = marker       // first differing byte outside BigEndianToUint64 range
	for i := 9; i < len(addr); i++ {
		addr[i] = 0x90 + marker
	}
	return sdk.ValAddress(addr)
}
