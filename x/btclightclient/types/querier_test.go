package types_test

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/babylonlabs-io/babylon/testutil/datagen"
	bbn "github.com/babylonlabs-io/babylon/types"
	"github.com/babylonlabs-io/babylon/x/btclightclient/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

func TestNewQueryHashesRequest(t *testing.T) {
	baseHeader := types.SimnetGenesisBlock()

	req := query.PageRequest{
		Key: baseHeader.Hash.MustMarshal(),
	}
	newQueryHashes := types.NewQueryHashesRequest(&req)
	if newQueryHashes == nil {
		t.Fatalf("A nil object was returned")
	}

	expectedQueryHashes := types.QueryHashesRequest{
		Pagination: &req,
	}
	if *newQueryHashes != expectedQueryHashes {
		t.Errorf("expected a QueryHashesRequest %s", expectedQueryHashes)
	}
}

func FuzzNewQueryContainsRequest(f *testing.F) {
	datagen.AddRandomSeedsToFuzzer(f, 10)
	f.Fuzz(func(t *testing.T, seed int64) {
		r := rand.New(rand.NewSource(seed))
		hexHash := datagen.GenRandomHexStr(r, bbn.BTCHeaderHashLen)

		btcHeaderHashBytes, _ := bbn.NewBTCHeaderHashBytesFromHex(hexHash)

		queryContains, err := types.NewQueryContainsRequest(hexHash)
		if err != nil {
			t.Errorf("returned error for valid hex %s", hexHash)
		}
		if queryContains == nil {
			t.Fatalf("returned a nil reference to a query")
		}
		if queryContains.Hash == nil {
			t.Errorf("has an empty hash attribute")
		}
		if !bytes.Equal(*(queryContains.Hash), btcHeaderHashBytes.MustMarshal()) {
			t.Errorf("expected hash bytes %s got %s", btcHeaderHashBytes.MustMarshal(), *(queryContains.Hash))
		}
	})
}

func TestNewQueryMainChainRequest(t *testing.T) {
	baseHeader := types.SimnetGenesisBlock()

	req := query.PageRequest{
		Key: baseHeader.Header.MustMarshal(),
	}
	newQueryMainChain := types.NewQueryMainChainRequest(&req)
	if newQueryMainChain == nil {
		t.Fatalf("A nil object was returned")
	}

	expectedQueryMainChain := types.QueryMainChainRequest{
		Pagination: &req,
	}
	if *newQueryMainChain != expectedQueryMainChain {
		t.Errorf("expected a QueryMainChainRequest %s", expectedQueryMainChain)
	}
}
