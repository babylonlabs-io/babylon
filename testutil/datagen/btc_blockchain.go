package datagen

import (
	"bytes"
	"math/big"
	"math/rand"
	"time"

	"github.com/babylonlabs-io/babylon/v4/btctxformatter"
	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcctypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	"github.com/btcsuite/btcd/blockchain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
)

// GenRandomBtcdBlock generates a random BTC block, which can include Babylon txs.
// Specifically,
// - when numBabylonTxs == 0 or numBabylonTxs > 2, it generates a BTC block with 3 random txs.
// - when numBabylonTxs == 1, it generates a BTC block with 2 random txs and a Babylon tx.
// - when numBabylonTxs == 2, it generates a BTC block with 1 random tx and 2 Babylon txs that constitute a raw BTC checkpoint.
// When numBabylonTxs == 2, the function will return the BTC raw checkpoint as well.
func GenRandomBtcdBlock(r *rand.Rand, numBabylonTxs int, prevHash *chainhash.Hash) (*wire.MsgBlock, *btctxformatter.RawBtcCheckpoint) {
	var (
		randomTxs []*wire.MsgTx                    = []*wire.MsgTx{GenRandomTx(r), GenRandomTx(r)}
		rawCkpt   *btctxformatter.RawBtcCheckpoint = nil
	)

	if numBabylonTxs == 2 {
		randomTxs, rawCkpt = GenRandomBabylonTxPair(r)
	} else if numBabylonTxs == 1 {
		bbnTxs, _ := GenRandomBabylonTxPair(r)
		idx := r.Intn(2)
		randomTxs[idx] = bbnTxs[idx]
	}
	coinbaseTx := createCoinbaseTx(r.Int31(), &chaincfg.SimNetParams)
	msgTxs := []*wire.MsgTx{coinbaseTx}
	msgTxs = append(msgTxs, randomTxs...)

	// calculate correct Merkle root
	merkleRoot := calcMerkleRoot(msgTxs)
	// don't apply any difficulty
	difficulty, _ := new(big.Int).SetString("7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 16)
	workBits := blockchain.BigToCompact(difficulty)

	header := GenRandomBtcdHeader(r)
	header.MerkleRoot = merkleRoot
	header.Bits = workBits
	if prevHash != nil {
		header.PrevBlock = *prevHash
	}
	// find a nonce that satisfies difficulty
	SolveBlock(header)

	block := &wire.MsgBlock{
		Header:       *header,
		Transactions: msgTxs,
	}
	return block, rawCkpt
}

// BTC block with proofs of each tx. Index of txs in the block is the same as the index of proofs.
type BlockWithProofs struct {
	Block        *wire.MsgBlock
	Proofs       []*btcctypes.BTCSpvProof
	Transactions []*wire.MsgTx
}

func GenRandomBtcdBlockWithTransactions(
	r *rand.Rand,
	txs []*wire.MsgTx,
	prevHeader *wire.BlockHeader,
) *BlockWithProofs {
	// we allways add coinbase tx to the block, to make sure merkle root is different
	// every time
	coinbaseTx := createCoinbaseTx(r.Int31(), &chaincfg.SimNetParams)
	msgTxs := []*wire.MsgTx{coinbaseTx}
	msgTxs = append(msgTxs, txs...)

	// calculate correct Merkle root
	merkleRoot := calcMerkleRoot(msgTxs)
	// don't apply any difficulty
	difficulty, _ := new(big.Int).SetString("7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", 16)
	workBits := blockchain.BigToCompact(difficulty)

	header := GenRandomBtcdHeader(r)
	header.MerkleRoot = merkleRoot
	header.Bits = workBits
	if prevHeader != nil {
		prevHash := prevHeader.BlockHash()
		header.PrevBlock = prevHash
		header.Timestamp = prevHeader.Timestamp.Add(time.Minute * 10)
	}
	// find a nonce that satisfies difficulty
	SolveBlock(header)

	var txBytes [][]byte
	for _, tx := range msgTxs {
		buf := bytes.NewBuffer(make([]byte, 0, tx.SerializeSize()))
		_ = tx.Serialize(buf)
		txBytes = append(txBytes, buf.Bytes())
	}

	var proofs []*btcctypes.BTCSpvProof

	for i := range msgTxs {
		headerBytes := bbn.NewBTCHeaderBytesFromBlockHeader(header)
		proof, err := btcctypes.SpvProofFromHeaderAndTransactions(&headerBytes, txBytes, uint(i))
		if err != nil {
			panic(err)
		}

		proofs = append(proofs, proof)
	}

	block := &wire.MsgBlock{
		Header:       *header,
		Transactions: msgTxs,
	}
	return &BlockWithProofs{
		Block:        block,
		Proofs:       proofs,
		Transactions: msgTxs,
	}
}

func GenChainFromListsOfTransactions(
	r *rand.Rand,
	txs [][]*wire.MsgTx,
	initHeader *wire.BlockHeader) []*BlockWithProofs {
	if initHeader == nil {
		panic("initHeader is required")
	}

	if len(txs) == 0 {
		panic("txs is required")
	}

	var parentHeader *wire.BlockHeader = initHeader
	var createdBlocks []*BlockWithProofs = []*BlockWithProofs{}
	for _, txs := range txs {
		msgBlock := GenRandomBtcdBlockWithTransactions(r, txs, parentHeader)
		parentHeader = &msgBlock.Block.Header
		createdBlocks = append(createdBlocks, msgBlock)
	}

	return createdBlocks
}

func GenNEmptyBlocks(r *rand.Rand, n uint64, prevHeader *wire.BlockHeader) []*BlockWithProofs {
	var txs [][]*wire.MsgTx = make([][]*wire.MsgTx, n)
	return GenChainFromListsOfTransactions(r, txs, prevHeader)
}

// GenRandomBtcdBlockchainWithBabylonTx generates a blockchain of `n` blocks, in which each block has some probability of including some Babylon txs
// Specifically, each block
// - has `oneTxThreshold` probability of including 1 Babylon tx that does not has any match
// - has `twoTxThreshold - oneTxThreshold` probability of including 2 Babylon txs that constitute a checkpoint
// - has `1 - twoTxThreshold` probability of including no Babylon tx
func GenRandomBtcdBlockchainWithBabylonTx(r *rand.Rand, n uint64, oneTxThreshold float32, twoTxThreshold float32) ([]*wire.MsgBlock, int, []*btctxformatter.RawBtcCheckpoint) {
	blocks := []*wire.MsgBlock{}
	numCkptSegs := 0
	rawCkpts := []*btctxformatter.RawBtcCheckpoint{}
	if oneTxThreshold < 0 || oneTxThreshold > 1 {
		panic("oneTxThreshold should be [0, 1]")
	}
	if twoTxThreshold < oneTxThreshold || twoTxThreshold > 1 {
		panic("fullPercentage should be [oneTxThreshold, 1]")
	}
	if n == 0 {
		panic("n should be > 0")
	}

	// genesis block
	genesisBlock, rawCkpt := GenRandomBtcdBlock(r, 0, nil)
	blocks = append(blocks, genesisBlock)
	rawCkpts = append(rawCkpts, rawCkpt)

	// blocks after genesis
	for i := uint64(1); i < n; i++ {
		var msgBlock *wire.MsgBlock
		prevHash := blocks[len(blocks)-1].BlockHash()

		switch {
		case r.Float32() < oneTxThreshold:
			msgBlock, rawCkpt = GenRandomBtcdBlock(r, 1, &prevHash)
			numCkptSegs += 1
		case r.Float32() < twoTxThreshold:
			msgBlock, rawCkpt = GenRandomBtcdBlock(r, 2, &prevHash)
			numCkptSegs += 2
		default:
			msgBlock, rawCkpt = GenRandomBtcdBlock(r, 0, &prevHash)
		}

		blocks = append(blocks, msgBlock)
		rawCkpts = append(rawCkpts, rawCkpt)
	}
	return blocks, numCkptSegs, rawCkpts
}

// GenRandomBtcdHash generates a random hash in type `chainhash.Hash`, without any hash operations
func GenRandomBtcdHash(r *rand.Rand) chainhash.Hash {
	hash, err := chainhash.NewHashFromStr(GenRandomHexStr(r, 32))
	if err != nil {
		panic(err)
	}
	return *hash
}
