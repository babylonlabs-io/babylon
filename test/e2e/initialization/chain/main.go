package main

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/babylonlabs-io/babylon/test/e2e/initialization"
	btclighttypes "github.com/babylonlabs-io/babylon/x/btclightclient/types"
)

func main() {
	var (
		valConfig             []*initialization.NodeConfig
		dataDir               string
		chainId               string
		config                string
		btcHeadersBytesHexStr string
		votingPeriod          time.Duration
		expeditedVotingPeriod time.Duration
		forkHeight            int
	)

	flag.StringVar(&dataDir, "data-dir", "", "chain data directory")
	flag.StringVar(&chainId, "chain-id", "", "chain ID")
	flag.StringVar(&config, "config", "", "serialized config")
	flag.StringVar(&btcHeadersBytesHexStr, "btc-headers", "", "btc header bytes comma separated")
	flag.DurationVar(&votingPeriod, "voting-period", 30000000000, "voting period")
	flag.DurationVar(&expeditedVotingPeriod, "expedited-voting-period", 20000000000, "expedited voting period")
	flag.IntVar(&forkHeight, "fork-height", 0, "fork height")

	flag.Parse()

	err := json.Unmarshal([]byte(config), &valConfig)
	if err != nil {
		panic(err)
	}

	if len(dataDir) == 0 {
		panic("data-dir is required")
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		panic(err)
	}

	btcHeaders := btcHeaderFromFlag(btcHeadersBytesHexStr)
	createdChain, err := initialization.InitChain(chainId, dataDir, valConfig, votingPeriod, expeditedVotingPeriod, forkHeight, btcHeaders)
	if err != nil {
		panic(err)
	}

	b, _ := json.Marshal(createdChain)
	fileName := fmt.Sprintf("%v/%v-encode", dataDir, chainId)
	if err = os.WriteFile(fileName, b, 0o777); err != nil {
		panic(err)
	}
}

func btcHeaderFromFlag(btcHeadersBytesHexStr string) []*btclighttypes.BTCHeaderInfo {
	btcHeaders := []*btclighttypes.BTCHeaderInfo{}
	if len(btcHeadersBytesHexStr) == 0 {
		return btcHeaders
	}

	btcHeadersBytesHex := strings.Split(btcHeadersBytesHexStr, ",")
	for _, btcHeaderBytesHex := range btcHeadersBytesHex {
		btcHeaderBytes, err := hex.DecodeString(btcHeaderBytesHex)
		if err != nil {
			panic(err)
		}

		btcHeader := &btclighttypes.BTCHeaderInfo{}
		err = btcHeader.Unmarshal(btcHeaderBytes)
		if err != nil {
			panic(err)
		}

		btcHeaders = append(btcHeaders, btcHeader)
	}
	return btcHeaders
}
