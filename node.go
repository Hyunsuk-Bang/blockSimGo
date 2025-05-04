package main

import (
	"container/heap"
	"fmt"
	"log"
	"math/rand"
)

type NodeStats struct {
	ReceivedTx         int
	AddedToMempool     int
	RelayedTx          int
	ReceivedBlocks     int
	ValidatedBlocks    int
	RelayedBlocks      int
	ReceivedOrphans    int
	ProcessedOrphans   int
	HandledReorgs      int
	StaleBlocksInReorg int
	MiningAttempts     int
	MinedBlocks        int
}

type Node struct {
	ID               int
	IsMiner          bool
	Peers            []int
	Mempool          map[string]Transaction
	KnownTx          map[string]bool
	Blocks           map[string]Block
	ChainHeight      map[int][]string
	BestChainTip     string
	ChainWork        map[string]int
	OrphanBlocks     map[string][]Block
	CurrentMiningJob *Event
	Sim              *Simulation
	Cfg              *Config
	Stats            NodeStats

	isWaitingToMine bool
}

func NewNode(id int, isMiner bool, sim *Simulation, cfg *Config) *Node {
	genesisBlock := sim.GenesisBlock
	n := &Node{
		ID:              id,
		IsMiner:         isMiner,
		Peers:           make([]int, 0),
		Mempool:         make(map[string]Transaction),
		KnownTx:         make(map[string]bool),
		Blocks:          make(map[string]Block),
		ChainHeight:     make(map[int][]string),
		ChainWork:       make(map[string]int),
		OrphanBlocks:    make(map[string][]Block),
		BestChainTip:    genesisBlock.Hash,
		Sim:             sim,
		Cfg:             cfg,
		Stats:           NodeStats{},
		isWaitingToMine: isMiner,
	}

	n.Blocks[genesisBlock.Hash] = genesisBlock
	n.ChainHeight[0] = []string{genesisBlock.Hash}
	n.ChainWork[genesisBlock.Hash] = 0
	n.KnownTx[genesisBlock.Hash] = true
	return n
}

func (n *Node) AddPeer(peerID int) {

	if !contains(n.Peers, peerID) {
		n.Peers = append(n.Peers, peerID)
	}
}

func (n *Node) ReceiveTransaction(tx Transaction) {
	n.Stats.ReceivedTx++
	if _, known := n.KnownTx[tx.ID]; known {
		return
	}

	n.Mempool[tx.ID] = tx
	n.KnownTx[tx.ID] = true
	n.Stats.AddedToMempool++

	if n.IsMiner && n.isWaitingToMine && n.CurrentMiningJob == nil {
		if n.canAttemptMiningNow() {
			log.Printf("T=%.3fs Node %d: Mempool reached threshold (95%%) after receiving Tx %s. Triggering mining attempt.\n",
				n.Sim.CurrentTime.Sub(n.Sim.StartTime).Seconds(), n.ID, tx.ID[:6])
			n.scheduleMiningAttempt()
			n.isWaitingToMine = false
		}
	}

	for targetNodeID := range n.Sim.Nodes {
		if targetNodeID == n.ID {
			continue
		}
		delay := CalculateNetworkDelay(n.Cfg)
		n.Sim.ScheduleEvent(n.Sim.CurrentTime.Add(delay), EvReceiveTransaction, ReceiveTransactionData{TargetNodeID: targetNodeID, Tx: tx})
		n.Stats.RelayedTx++
	}
}

func (n *Node) ReceiveBlock(b Block) {
	n.Stats.ReceivedBlocks++
	if _, known := n.Blocks[b.Hash]; known {
		return
	}

	parentBlock, parentKnown := n.Blocks[b.Header.PrevHash]
	if !parentKnown {
		n.Stats.ReceivedOrphans++
		n.OrphanBlocks[b.Header.PrevHash] = append(n.OrphanBlocks[b.Header.PrevHash], b)
		return
	}
	if b.Header.Height != parentBlock.Header.Height+1 {
		return
	}
	n.Stats.ValidatedBlocks++

	n.Blocks[b.Hash] = b
	n.KnownTx[b.Hash] = true
	n.ChainHeight[b.Header.Height] = append(n.ChainHeight[b.Header.Height], b.Hash)
	n.ChainWork[b.Hash] = n.ChainWork[b.Header.PrevHash] + 1

	n.updateMempoolForNewBlock(b)

	if orphans, found := n.OrphanBlocks[b.Hash]; found {
		n.Stats.ProcessedOrphans += len(orphans)
		delete(n.OrphanBlocks, b.Hash)
		for _, orphanBlock := range orphans {
			n.Sim.ScheduleEventWithPriority(n.Sim.CurrentTime, EvReceiveBlock, ReceiveBlockData{TargetNodeID: n.ID, Block: orphanBlock}, 0)
		}
	}

	currentBestHeight := n.ChainWork[n.BestChainTip]
	newBlockHeight := n.ChainWork[b.Hash]

	if newBlockHeight > currentBestHeight {
		oldTipHash := n.BestChainTip
		n.BestChainTip = b.Hash

		if b.Header.PrevHash != oldTipHash {
			n.handleReorg(oldTipHash, b.Hash)
		}

		n.relayBlock(b)
		n.restartMining()

	} else {
		n.relayBlock(b)
	}
}

func (n *Node) relayBlock(b Block) {

	for targetNodeID := range n.Sim.Nodes {
		if targetNodeID == n.ID {
			continue
		}
		delay := CalculateNetworkDelay(n.Cfg)
		n.Sim.ScheduleEvent(n.Sim.CurrentTime.Add(delay), EvReceiveBlock, ReceiveBlockData{
			TargetNodeID: targetNodeID, Block: b,
		})
		n.Stats.RelayedBlocks++
	}

}

func (n *Node) restartMining() {
	if !n.IsMiner {
		return
	}

	if n.CurrentMiningJob != nil {
		n.CurrentMiningJob = nil
	}
	n.isWaitingToMine = false

	if n.canAttemptMiningNow() {

		n.scheduleMiningAttempt()
	} else {
		log.Printf("T=%.3fs Node %d: Mempool below threshold (95%%) upon block update. Waiting for transactions.\n",
			n.Sim.CurrentTime.Sub(n.Sim.StartTime).Seconds(), n.ID)
		n.isWaitingToMine = true
	}
}

func (n *Node) canAttemptMiningNow() bool {
	if !n.IsMiner {
		return false
	}

	requiredBytesFloat := float64(n.Cfg.BlockSizeLimitBytes) * (95.0 / 100.0)

	if requiredBytesFloat <= 0 {
		return true
	}

	currentMempoolTotalBytes := 0
	for _, tx := range n.Mempool {
		currentMempoolTotalBytes += tx.Size
	}

	return float64(currentMempoolTotalBytes) >= requiredBytesFloat
}

func (n *Node) scheduleMiningAttempt() {

	if n.CurrentMiningJob != nil {
		return
	}

	tipHeight := n.ChainWork[n.BestChainTip]
	parentHash := n.BestChainTip
	nextHeight := tipHeight + 1

	log.Printf("T=%.3fs Node %d: Scheduling mining attempt for height %d on parent %s\n",
		n.Sim.CurrentTime.Sub(n.Sim.StartTime).Seconds(), n.ID, nextHeight, parentHash[:6])

	n.Sim.ScheduleEvent(n.Sim.CurrentTime, EvAttemptMining, AttemptMiningData{
		MinerNodeID:     n.ID,
		ParentBlockHash: parentHash,
		Height:          nextHeight,
	})

}

func (n *Node) AttemptMining(data AttemptMiningData) {
	if !n.IsMiner {
		return
	}

	if data.ParentBlockHash != n.BestChainTip {
		return
	}
	if data.Height != n.ChainWork[n.BestChainTip]+1 {
		return
	}

	n.Stats.MiningAttempts++

	selectedTxs := []Transaction{}
	currentBlockSizeBytes := 0
	mempoolTxs := make([]Transaction, 0, len(n.Mempool))
	for _, tx := range n.Mempool {
		mempoolTxs = append(mempoolTxs, tx)
	}
	rand.Shuffle(len(mempoolTxs), func(i, j int) { mempoolTxs[i], mempoolTxs[j] = mempoolTxs[j], mempoolTxs[i] })

	for _, tx := range mempoolTxs {
		if currentBlockSizeBytes+tx.Size <= n.Cfg.BlockSizeLimitBytes {
			selectedTxs = append(selectedTxs, tx)
			currentBlockSizeBytes += tx.Size
		}
	}

	log.Printf("T=%.3fs Node %d: Starting Mining Calculation H=%d Parent=%s | Selected=%d txs (%d bytes / %d limit)",
		n.Sim.CurrentTime.Sub(n.Sim.StartTime).Seconds(), n.ID, data.Height, data.ParentBlockHash[:6],
		len(selectedTxs), currentBlockSizeBytes, n.Cfg.BlockSizeLimitBytes)

	candidateBlock := NewBlock(data.Height, data.ParentBlockHash, n.Sim.CurrentTime, n.ID, selectedTxs)
	timeToFind := CalculateTimeToFind(n.Cfg)
	foundTimestamp := n.Sim.CurrentTime.Add(timeToFind)

	if foundTimestamp.Sub(n.Sim.StartTime) < n.Cfg.SimulationDuration {
		foundEvent := &Event{
			Timestamp: foundTimestamp, Type: EvBlockFound,
			Data:     BlockFoundData{MinerNodeID: n.ID, Block: candidateBlock},
			Priority: int(EvBlockFound),
		}
		heap.Push(&n.Sim.EventQueue, foundEvent)
		n.CurrentMiningJob = foundEvent
	} else {
		n.CurrentMiningJob = nil
	}
}

func (n *Node) ProcessFoundBlock(data BlockFoundData, event *Event) {
	if !n.IsMiner {
		return
	}

	if n.CurrentMiningJob == event {
		n.CurrentMiningJob = nil
		n.Stats.MinedBlocks++

		foundBlock := data.Block
		foundBlock.FoundTime = event.Timestamp

		blockFoundTime := event.Timestamp
		for _, tx := range foundBlock.Transactions {
			if meta, exists := n.Sim.TxStatus[tx.ID]; exists {
				if meta.IncludedInBlock == "" {
					meta.IncludedInBlock = foundBlock.Hash
					meta.FirstBlockTime = blockFoundTime
				}
			}
		}

		n.ReceiveBlock(foundBlock)

	} else {

	}
}

func (n *Node) updateMempoolForNewBlock(b Block) {
	for _, tx := range b.Transactions {
		delete(n.Mempool, tx.ID)
		n.KnownTx[tx.ID] = true
	}
}

func (n *Node) handleReorg(oldTipHash string, newTipHash string) {
	n.Stats.HandledReorgs++

	ancestorHash := n.findCommonAncestor(oldTipHash, newTipHash)
	if ancestorHash == "" {
		log.Printf("...")
		return
	}

	staleBlocks := []Block{}
	currentHash := oldTipHash
	initialStaleCount := n.Stats.StaleBlocksInReorg
	for currentHash != ancestorHash {
		block, ok := n.Blocks[currentHash]
		if !ok {
			break
		}
		staleBlocks = append(staleBlocks, block)
		n.Stats.StaleBlocksInReorg++
		currentHash = block.Header.PrevHash
	}

	n.Sim.IncrementStaleCounterBy(n.Stats.StaleBlocksInReorg - initialStaleCount)

	newBlocks := []Block{}
	currentHash = newTipHash
	for currentHash != ancestorHash {
		block, ok := n.Blocks[currentHash]
		if !ok {
			break
		}
		newBlocks = append([]Block{block}, newBlocks...)
		currentHash = block.Header.PrevHash
	}

	for _, staleBlock := range staleBlocks {
		for _, tx := range staleBlock.Transactions {
			inNewChain := false
			for _, newBlock := range newBlocks {
				for _, newTx := range newBlock.Transactions {
					if tx.ID == newTx.ID {
						inNewChain = true
						break
					}
				}
				if inNewChain {
					break
				}
			}
			if !inNewChain {

				if _, known := n.KnownTx[tx.ID]; !known {
					n.Mempool[tx.ID] = tx
					n.KnownTx[tx.ID] = true
				} else if _, inMempool := n.Mempool[tx.ID]; !inMempool {
					n.Mempool[tx.ID] = tx
				}
			}
		}
	}

}

func (n *Node) findCommonAncestor(hash1, hash2 string) string {
	h1, ok1 := n.ChainWork[hash1]
	h2, ok2 := n.ChainWork[hash2]
	if !ok1 || !ok2 {
		return ""
	}

	curr1, curr2 := hash1, hash2

	for h1 > h2 {
		b, ok := n.Blocks[curr1]
		if !ok {
			return ""
		}
		curr1 = b.Header.PrevHash
		h1--
	}
	for h2 > h1 {
		b, ok := n.Blocks[curr2]
		if !ok {
			return ""
		}
		curr2 = b.Header.PrevHash
		h2--
	}

	for curr1 != curr2 {
		b1, ok1 := n.Blocks[curr1]
		b2, ok2 := n.Blocks[curr2]
		if !ok1 || !ok2 {
			return ""
		}
		if b1.Header.PrevHash == "" || b2.Header.PrevHash == "" {
			return n.Sim.GenesisBlock.Hash
		}
		curr1 = b1.Header.PrevHash
		curr2 = b2.Header.PrevHash
	}
	return curr1
}

func (n *Node) PrintStats() {
	tipHeight := -1
	if h, ok := n.ChainWork[n.BestChainTip]; ok {
		tipHeight = h
	} else if n.BestChainTip == n.Sim.GenesisBlock.Hash {
		tipHeight = 0
	}

	fmt.Printf("--- Node %d Stats ---\n", n.ID)
	fmt.Printf("  Final State: Tip=%s (Height:%d), MempoolSize:%d txs\n",
		n.BestChainTip[:6], tipHeight, len(n.Mempool))
	fmt.Printf("  Transactions: Rcvd:%d, AddedToMempool:%d, Relayed/Bcast:%d\n",
		n.Stats.ReceivedTx, n.Stats.AddedToMempool, n.Stats.RelayedTx)
	fmt.Printf("  Blocks: Rcvd:%d, Validated:%d, Relayed/Bcast:%d\n",
		n.Stats.ReceivedBlocks, n.Stats.ValidatedBlocks, n.Stats.RelayedBlocks)
	fmt.Printf("  Orphans: Rcvd:%d, ProcessedLater:%d\n",
		n.Stats.ReceivedOrphans, n.Stats.ProcessedOrphans)
	fmt.Printf("  Forks: ReorgsHandled:%d, StaleBlocksInReorgs:%d\n",
		n.Stats.HandledReorgs, n.Stats.StaleBlocksInReorg)
	if n.IsMiner {
		fmt.Printf("  Mining: AttemptsStarted:%d, BlocksMinedSuccess:%d\n",
			n.Stats.MiningAttempts, n.Stats.MinedBlocks)
	}
}
