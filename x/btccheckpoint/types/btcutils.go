package types

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/babylonlabs-io/babylon/types"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
)

// ParsedProof represent semantically valid:
// - Bitcoin Header
// - Bitcoin Header hash
// - Bitcoin Transaction
// - Bitcoin Transaction index in block
// - Non-empty OpReturnData
type ParsedProof struct {
	// keeping header hash to avoid recomputing it everytime
	BlockHash        types.BTCHeaderHashBytes
	Transaction      *btcutil.Tx
	TransactionBytes []byte
	TransactionIdx   uint32
	OpReturnData     []byte
}

// Concatenates and double hashes two provided inputs
func hashConcat(a []byte, b []byte) chainhash.Hash {
	c := []byte{}
	c = append(c, a...)
	c = append(c, b...)
	return chainhash.DoubleHashH(c)
}

func min(a, b uint) uint {
	if a < b {
		return a
	}
	return b
}

// createBranch takes as input flatenned representation of merkle tree i.e
// for tree:
//
//	      r
//	    /  \
//	  d1    d2
//	 /  \   / \
//	l1  l2 l3 l4
//
// slice should look like [l1, l2, l3, l4, d1, d2, r]
// also it takes number of leafs i.e nodes at lowest level of the tree and index
// of the leaf which supposed to be proven
// it returns list of hashes required to prove given index
func createBranch(nodes []*chainhash.Hash, numfLeafs uint, idx uint) []*chainhash.Hash {

	var branch []*chainhash.Hash

	// size represents number of merkle nodes at given level. At 0 level, number of
	// nodes is equal to number of leafs
	var size = numfLeafs

	var index = idx

	// i represents starting index of given level. 0 level i.e leafs always start
	// at index 0
	var i uint = 0

	for size > 1 {

		// index^1 means we want to get sibling of the node we are proving
		// ie. for index=2, index^1 = 3 and for index=3 index^1=2
		// so xoring last bit by 1, select node oposite to the node we want the proof
		// for.
		// case with `size-1` is needed when the number of leafs is not power of 2
		// and xor^1 could point outside of the tree
		j := min(index^1, size-1)

		branch = append(branch, nodes[i+j])

		// divide index by 2 as there are two times less nodes on second level
		index = index >> 1

		// after getting node at this level we move to next one by advancing i by
		// the size of the current level
		i = i + size

		// update the size to the next level size i.e (current level size / 2)
		// + 1 is needed to correctly account for cases that the last node of the level
		// is not paired.
		// example If the level is of the size 3, then next level should have size 2, not 1
		size = (size + 1) >> 1
	}

	return branch
}

// quite inefficiet method of calculating merkle proofs, created for testing purposes
func CreateProofForIdx(transactions [][]byte, idx uint) ([]*chainhash.Hash, error) {
	if len(transactions) == 0 {
		return nil, errors.New("can't calculate proof for empty transaction list")
	}

	if int(idx) >= len(transactions) {
		return nil, errors.New("provided index should be smaller that lenght of transaction list")
	}

	var txs []*btcutil.Tx
	for _, b := range transactions {
		tx, e := btcutil.NewTxFromBytes(b)

		if e != nil {
			return nil, e
		}

		txs = append(txs, tx)
	}

	store := blockchain.BuildMerkleTreeStore(txs, false)

	var storeNoNil []*chainhash.Hash

	// to correctly calculate indexes we need to filter out all nil hashes which
	// represents nodes which are empty
	for _, h := range store {
		if h != nil {
			storeNoNil = append(storeNoNil, h)
		}
	}

	branch := createBranch(storeNoNil, uint(len(transactions)), idx)

	return branch, nil
}

// Verify checks the validity of a merkle proof
// proof logic copied from:
// https://github.com/summa-tx/bitcoin-spv/blob/fb2a61e7a941d421ae833789d97ed10d2ad79cfe/golang/btcspv/bitcoin_spv.go#L498
// main reason for not bringing library in, is that we already use btcd
// bitcoin primitives and this library defines their own which could lead
// to some mixups
func verify(tx *btcutil.Tx, merkleRoot *chainhash.Hash, intermediateNodes []byte, index uint32) bool {
	txHash := tx.Hash()

	// Shortcut the empty-block case
	if txHash.IsEqual(merkleRoot) && index == 0 && len(intermediateNodes) == 0 {
		return true
	}

	proof := []byte{}
	proof = append(proof, txHash[:]...)
	proof = append(proof, intermediateNodes...)
	proof = append(proof, merkleRoot[:]...)

	var current chainhash.Hash

	idx := index

	proofLength := len(proof)

	if proofLength%32 != 0 {
		return false
	}

	if proofLength == 64 {
		return false
	}

	root := proof[proofLength-32:]

	cur := proof[:32:32]
	copy(current[:], cur)

	numSteps := (proofLength / 32) - 1

	for i := 1; i < numSteps; i++ {
		start := i * 32
		end := i*32 + 32
		next := proof[start:end:end]
		if idx%2 == 1 {
			current = hashConcat(next, current[:])
		} else {
			current = hashConcat(current[:], next)
		}
		idx >>= 1
	}

	return bytes.Equal(current[:], root)
}

func VerifyInclusionProof(
	tx *btcutil.Tx,
	merkleRoot *chainhash.Hash,
	intermediateNodes []byte,
	index uint32) bool {
	return verify(tx, merkleRoot, intermediateNodes, index)
}

// ExtractStandardOpReturnData extract OP_RETURN data from transaction OP_RETURN
// output.
// If OP_RETURN output is not standard it will be ignored. If there is more than
// one output with OP_RETURN, error will be returned.
func ExtractStandardOpReturnData(tx *btcutil.Tx) ([]byte, error) {
	msgTx := tx.MsgTx()
	opReturnData := []byte{}

	var opReturnCounter = 0

	for _, output := range msgTx.TxOut {
		script := output.PkScript

		if !txscript.IsNullData(script) {
			// not a standard op_return, we do not care about this output
			continue
		}
		// At this point we know:
		// - script is not empty
		// - script is valid looking op_return
		// - with at most 80bytes of data
		opReturnCounter++

		if opReturnCounter > 1 {
			return nil, fmt.Errorf("transaction has more than one OP_RETURN output")
		}

		if len(script) == 1 {
			// just op_return op code
			continue
		}

		if script[1] == txscript.OP_PUSHDATA1 {
			// we need to drop first 3 bytes as those are related
			// to script iteslf i.e OP_RETURN + OP_PUSHDATA1 + len of bytes
			opReturnData = append(opReturnData, script[3:]...)
		} else {
			// this should be one of OP_DATAXX opcodes we drop first 2 bytes
			opReturnData = append(opReturnData, script[2:]...)
		}
	}

	return opReturnData, nil
}

func ParseTransaction(bytes []byte) (*btcutil.Tx, error) {
	tx, e := btcutil.NewTxFromBytes(bytes)

	if e != nil {
		return nil, e
	}

	e = blockchain.CheckTransactionSanity(tx)

	if e != nil {
		return nil, e
	}

	return tx, nil
}

// TODO define domain errors with nice error messages
// TODO add some tests for the proof validation
func ParseProof(
	btcTransaction []byte,
	transactionIndex uint32,
	merkleProof []byte,
	btcHeader *types.BTCHeaderBytes,
	powLimit *big.Int) (*ParsedProof, error) {
	tx, e := ParseTransaction(btcTransaction)

	if e != nil {
		return nil, e
	}

	header := btcHeader.ToBlockHeader()

	e = types.ValidateBTCHeader(header, powLimit)

	if e != nil {
		return nil, e
	}

	validProof := verify(tx, &header.MerkleRoot, merkleProof, transactionIndex)

	if !validProof {
		return nil, fmt.Errorf("header failed validation due to failed proof")
	}

	opReturnData, err := ExtractStandardOpReturnData(tx)

	if err != nil {
		return nil, err
	}

	if len(opReturnData) == 0 {
		return nil, fmt.Errorf("provided transaction should provide op return data")
	}

	bh := header.BlockHash()
	parsedProof := &ParsedProof{
		BlockHash:        types.NewBTCHeaderHashBytesFromChainhash(&bh),
		Transaction:      tx,
		TransactionBytes: btcTransaction,
		TransactionIdx:   transactionIndex,
		OpReturnData:     opReturnData,
	}

	return parsedProof, nil
}

// TODO: tests and benchmarking on this function
func SpvProofFromHeaderAndTransactions(
	headerBytes *types.BTCHeaderBytes,
	transactions [][]byte,
	transactionIdx uint,
) (*BTCSpvProof, error) {
	proof, e := CreateProofForIdx(transactions, transactionIdx)

	if e != nil {
		return nil, e
	}

	var flatProof []byte

	for _, h := range proof {
		flatProof = append(flatProof, h.CloneBytes()...)
	}

	spvProof := BTCSpvProof{
		BtcTransaction:      transactions[transactionIdx],
		BtcTransactionIndex: uint32(transactionIdx),
		MerkleNodes:         flatProof,
		ConfirmingBtcHeader: headerBytes,
	}

	return &spvProof, nil
}
