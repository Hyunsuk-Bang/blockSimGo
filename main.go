package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func main() {
	log.Printf("--- Starting Simulation Setup ---")

	log.Println("Parsing configuration flags...")
	cfg := DefaultConfig()
	flag.IntVar(&cfg.NumNodes, "nodes", cfg.NumNodes, "Total number of nodes")
	flag.IntVar(&cfg.NumMiners, "miners", cfg.NumMiners, "Number of mining nodes")
	flag.IntVar(&cfg.BlockSizeLimitBytes, "block_size_bytes", cfg.BlockSizeLimitBytes, "Max block size in bytes")
	flag.Float64Var(&cfg.TransactionRatePerSec, "tx_rate", cfg.TransactionRatePerSec, "Transaction injection rate per second")
	flag.IntVar(&cfg.MinTransactionSizeBytes, "tx_size_min", cfg.MinTransactionSizeBytes, "CLAMP: Minimum transaction size in bytes")
	flag.IntVar(&cfg.MaxTransactionSizeBytes, "tx_size_max", cfg.MaxTransactionSizeBytes, "CLAMP: Maximum transaction size in bytes")
	flag.Float64Var(&cfg.MeanTransactionSizeBytes, "tx_size_mean", cfg.MeanTransactionSizeBytes, "Mean transaction size in bytes (Normal Dist)")
	flag.Float64Var(&cfg.StdDevTransactionSizeBytes, "tx_size_stddev", cfg.StdDevTransactionSizeBytes, "Standard Deviation for transaction size (Normal Dist)")
	flag.DurationVar(&cfg.NetworkDelayMin, "delay_min", cfg.NetworkDelayMin, "Minimum network delay")
	flag.DurationVar(&cfg.NetworkDelayMax, "delay_max", cfg.NetworkDelayMax, "Maximum network delay")
	flag.IntVar(&cfg.TotalInputTransactions, "total_txs", cfg.TotalInputTransactions, "Target total input transactions to inject")
	flag.DurationVar(&cfg.SimulationDuration, "duration", cfg.SimulationDuration, "Maximum simulation duration")
	flag.DurationVar(&cfg.FindTimeMin, "find_time_min", cfg.FindTimeMin, "Minimum time to find a block")
	flag.DurationVar(&cfg.FindTimeMax, "find_time_max", cfg.FindTimeMax, "Maximum time to find a block")

	flag.Parse()
	log.Println("Flag parsing complete.")

	if cfg.NumMiners > cfg.NumNodes {
		log.Fatalf("Error: Number of miners (%d) cannot exceed number of nodes (%d)", cfg.NumMiners, cfg.NumNodes)
	}
	if cfg.NetworkDelayMin > cfg.NetworkDelayMax {
		log.Fatalf("Error: Minimum network delay (%v) cannot exceed maximum network delay (%v)", cfg.NetworkDelayMin, cfg.NetworkDelayMax)
	}
	if cfg.NumMiners <= 0 && cfg.NumNodes > 0 {
		log.Println("Warning: No miners specified. Blockchain will likely not progress.")
	}
	if cfg.SimulationDuration <= 0 {
		log.Fatalf("Error: Simulation duration (%v) must be positive.", cfg.SimulationDuration)
	}
	if cfg.BlockSizeLimitBytes <= 0 {
		log.Fatalf("Error: Block size limit (%d) must be positive.", cfg.BlockSizeLimitBytes)
	}
	if cfg.MinTransactionSizeBytes <= 0 {
		log.Fatalf("Error: Min transaction size must be positive.")
	}
	if cfg.MaxTransactionSizeBytes < cfg.MinTransactionSizeBytes {
		log.Fatalf("Error: Max transaction size cannot be less than min transaction size.")
	}
	cfg.TargetBlockInterval = (cfg.FindTimeMin + cfg.FindTimeMax) / 2.0

	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	log.Println("--- Blockchain Simulator ---")
	log.Printf("Config: %+v\n", cfg)

	sim := NewSimulation(cfg)
	sim.Run()

	log.Println("--- Simulation Results ---")
	actualDuration := sim.CurrentTime.Sub(sim.StartTime)
	log.Printf("Simulation Stopped At: %.3f seconds (Target Duration: %v)\n", actualDuration.Seconds(), cfg.SimulationDuration)
	log.Printf("Global Stale Blocks Count: %d\n", sim.GlobalStaleCount)
	log.Printf("Total Transactions Injected: %d / %d (target)\n", sim.TxSource.GeneratedCount, cfg.TotalInputTransactions)

	log.Printf("--- Final Chain Analysis (based on Node 0 at T=%.3fs) ---", actualDuration.Seconds())
	mainChainBlocks, err := getMainChainBlocks(sim, 0)
	if err != nil {
		log.Printf("Could not analyze main chain details: %v\n", err)
	} else {

		avgInterval, errInterval := calculateAverageBlockInterval(mainChainBlocks)
		if errInterval != nil {
			log.Printf("Could not calculate average block interval: %v\n", errInterval)
		} else {
			log.Printf("Average Actual Block Interval: %v (Target: %v)\n", avgInterval, cfg.TargetBlockInterval)
		}

		avgBlockTPS := calculateBlockBasedThroughput(mainChainBlocks, &cfg)
		log.Printf("Average Block Throughput: %.2f TPS (Avg(Block Txs / Target Interval))\n", avgBlockTPS)
	}

	checkChainConsensus(sim)
	printFinalBlockchain(sim, 0)
	log.Println("--- Simulation Complete ---")
}

func checkChainConsensus(sim *Simulation) {
	if len(sim.Nodes) == 0 {
		return
	}
	tipCounts := make(map[string]int)
	maxHeight := -1
	consensusTip := ""
	consensusHeight := -1
	for _, node := range sim.Nodes {
		tipHash := node.BestChainTip
		tipHeight := -1
		if workHeight, ok := node.ChainWork[tipHash]; ok {
			tipHeight = workHeight
		} else if tipHash == sim.GenesisBlock.Hash {
			tipHeight = 0
		}
		tipCounts[tipHash]++
		if tipHeight > maxHeight {
			maxHeight = tipHeight
		}
	}
	log.Printf("--- Chain Consensus Check (Final State) ---")
	log.Printf("Max Height Reached (any node): %d", maxHeight)
	if len(tipCounts) == 1 {
		for tip := range tipCounts {
			consensusTip = tip
		}
		if workHeight, ok := sim.Nodes[0].ChainWork[consensusTip]; ok {
			consensusHeight = workHeight
		} else if consensusTip == sim.GenesisBlock.Hash {
			consensusHeight = 0
		}
		log.Printf("All %d nodes agree on final tip: %s (Height: %d)", len(sim.Nodes), consensusTip[:6], consensusHeight)
	} else {
		log.Printf("Nodes disagree on final tip:")
		for tip, count := range tipCounts {
			height := -1
			for _, node := range sim.Nodes {
				if node.BestChainTip == tip {
					if workHeight, ok := node.ChainWork[tip]; ok {
						height = workHeight
					} else if tip == sim.GenesisBlock.Hash {
						height = 0
					}
					break
				}
			}
			log.Printf("  - Tip: %s (Height: %d) agreed by %d node(s)", tip[:6], height, count)
		}
	}
}

func getMainChainBlocks(sim *Simulation, referenceNodeID int) ([]Block, error) {

	node, ok := sim.Nodes[referenceNodeID]
	if !ok {
		return nil, fmt.Errorf("reference Node %d not found", referenceNodeID)
	}
	finalTipHash := node.BestChainTip
	if finalTipHash == "" {
		if sim.GenesisBlock.Hash != "" {
			if _, exists := node.Blocks[sim.GenesisBlock.Hash]; exists {
				return []Block{sim.GenesisBlock}, nil
			}
		}
		return nil, fmt.Errorf("reference Node %d has an empty best chain tip", referenceNodeID)
	}
	mainChain := []Block{}
	currentHash := finalTipHash
	blocksToFetch := 0
	expectedHeight := -1
	if h, ok := node.ChainWork[finalTipHash]; ok {
		expectedHeight = h
	} else if finalTipHash == sim.GenesisBlock.Hash {
		expectedHeight = 0
	}
	for currentHash != "" {
		block, exists := node.Blocks[currentHash]
		if !exists {
			return nil, fmt.Errorf("block %s missing in Node %d's view during chain traversal", currentHash[:6], referenceNodeID)
		}
		mainChain = append(mainChain, block)
		if currentHash == sim.GenesisBlock.Hash {
			break
		}
		currentHash = block.Header.PrevHash
		blocksToFetch++
		limit := 100000
		if expectedHeight >= 0 {
			limit = expectedHeight + 10
		}
		if blocksToFetch > limit {
			return nil, fmt.Errorf("traversed too many blocks (%d) while getting chain. Aborting", blocksToFetch)
		}
	}
	if currentHash != sim.GenesisBlock.Hash && blocksToFetch > 0 {
		return nil, fmt.Errorf("chain traversal ended unexpectedly before reaching Genesis (last hash: %s)", mainChain[len(mainChain)-1].Hash[:6])
	}
	for i, j := 0, len(mainChain)-1; i < j; i, j = i+1, j-1 {
		mainChain[i], mainChain[j] = mainChain[j], mainChain[i]
	}
	return mainChain, nil
}

func calculateAverageBlockInterval(mainChain []Block) (time.Duration, error) {
	if len(mainChain) < 2 {
		return 0, errors.New("need at least two blocks to calculate an interval")
	}
	var totalInterval time.Duration
	intervalCount := 0
	for i := 1; i < len(mainChain); i++ {
		prevBlock := mainChain[i-1]
		currBlock := mainChain[i]
		if prevBlock.FoundTime.IsZero() || currBlock.FoundTime.IsZero() {
			continue
		}
		interval := currBlock.FoundTime.Sub(prevBlock.FoundTime)
		if interval < 0 {
			continue
		}
		totalInterval += interval
		intervalCount++
	}
	if intervalCount == 0 {
		return 0, errors.New("no valid block intervals found to average")
	}
	averageInterval := totalInterval / time.Duration(intervalCount)
	return averageInterval, nil
}

func calculateBlockBasedThroughput(mainChain []Block, cfg *Config) float64 {
	if len(mainChain) <= 1 {
		return 0.0
	}
	intervalSeconds := cfg.TargetBlockInterval.Seconds()
	if intervalSeconds <= 0 {
		return 0.0
	}
	totalRateSum := 0.0
	blockCount := 0
	for _, block := range mainChain {
		if block.Header.Height == 0 {
			continue
		}
		blockTxRate := float64(block.Header.NumTx) / intervalSeconds
		totalRateSum += blockTxRate
		blockCount++
	}
	if blockCount == 0 {
		return 0.0
	}
	averageRate := totalRateSum / float64(blockCount)
	return averageRate
}

func printFinalBlockchain(sim *Simulation, referenceNodeID int) {
	log.Println("--- Final Blockchain (Node", referenceNodeID, " View) ---")
	mainChainBlocks, err := getMainChainBlocks(sim, referenceNodeID)
	if err != nil {
		log.Printf("Error getting main chain: %v", err)
		return
	}
	if len(mainChainBlocks) == 0 {
		log.Println("Main chain is empty.")
		return
	}
	for i, block := range mainChainBlocks {
		indent := ""
		if i > 0 {
			indent = "  ->"
		}

		var blockByteSize int = 0
		for _, tx := range block.Transactions {
			blockByteSize += tx.Size
		}

		fmt.Printf("%s Block Height: %d | Hash: %s | Miner: %d | Time: %s | Txs: %d | Size: %d\n",
			indent, block.Header.Height, block.Hash[:10], block.Header.MinerID,
			block.Header.Timestamp.Format(time.StampMilli),
			block.Header.NumTx, blockByteSize,
		)
	}
	log.Println("--- End of Blockchain ---")
}
