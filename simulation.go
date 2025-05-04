package main

import (
	"container/heap"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
)

type TxMetadata struct {
	InjectTime      time.Time
	FirstBlockTime  time.Time
	ConfirmedTime   time.Time
	IncludedInBlock string
	IsConfirmed     bool
}

type Simulation struct {
	Cfg              *Config
	Nodes            map[int]*Node
	EventQueue       EventQueue
	CurrentTime      time.Time
	StartTime        time.Time
	GlobalStaleCount int
	MinerIDs         []int
	TxSource         *SimpleTxSource
	AllInputTxHashes map[string]bool
	TxStatus         map[string]*TxMetadata
	ProcessedTxCount int
	GenesisBlock     Block
}

func NewSimulation(cfg Config) *Simulation {
	genesisTime := time.Now()
	genesis := Block{
		Header:       BlockHeader{Height: 0, Timestamp: genesisTime, PrevHash: strings.Repeat("0", 64), MinerID: -1, NumTx: 0},
		Transactions: []Transaction{},
	}
	genesis.Hash = genesis.CalculateHash()
	genesis.FoundTime = genesisTime

	sim := &Simulation{
		Cfg:              &cfg,
		Nodes:            make(map[int]*Node),
		EventQueue:       make(EventQueue, 0),
		StartTime:        genesisTime,
		CurrentTime:      genesisTime,
		GlobalStaleCount: 0,
		MinerIDs:         make([]int, 0),
		AllInputTxHashes: make(map[string]bool),
		TxStatus:         make(map[string]*TxMetadata),
		TxSource:         NewSimpleTxSource(&cfg, genesisTime),
		GenesisBlock:     genesis,
		ProcessedTxCount: 0,
	}
	heap.Init(&sim.EventQueue)
	return sim
}

func (s *Simulation) ScheduleEvent(t time.Time, et EventType, data interface{}) {
	s.ScheduleEventWithPriority(t, et, data, int(et))
}

func (s *Simulation) ScheduleEventWithPriority(t time.Time, et EventType, data interface{}, priority int) {
	event := &Event{Timestamp: t, Type: et, Data: data, Priority: priority}
	heap.Push(&s.EventQueue, event)
}

func (s *Simulation) IncrementStaleCounterBy(count int) {
	s.GlobalStaleCount += count
}

func (s *Simulation) Setup() {
	log.Println("Setting up simulation...")
	minerCount := 0
	nodeIDs := rand.Perm(s.Cfg.NumNodes)
	for i := 0; i < s.Cfg.NumNodes; i++ {
		nodeID := i
		isMiner := false
		isMinerCandidate := false
		originalIndex := nodeIDs[i]
		if originalIndex < s.Cfg.NumMiners {
			isMinerCandidate = true
		}
		if isMinerCandidate && minerCount < s.Cfg.NumMiners {
			isMiner = true
			s.MinerIDs = append(s.MinerIDs, nodeID)
			minerCount++
		}

		s.Nodes[nodeID] = NewNode(nodeID, isMiner, s, s.Cfg)
	}

	for i := 0; i < s.Cfg.NumNodes; i++ {
		numPeersToAttempt := 3 + rand.Intn(3)
		peersConnected := 0
		attemptCounter := 0
		for peersConnected < numPeersToAttempt && len(s.Nodes[i].Peers) < s.Cfg.NumNodes-1 {
			peerID := rand.Intn(s.Cfg.NumNodes)
			if peerID != i && !contains(s.Nodes[i].Peers, peerID) {
				s.Nodes[i].AddPeer(peerID)
				if !contains(s.Nodes[peerID].Peers, i) {
					s.Nodes[peerID].AddPeer(i)
				}
				peersConnected++
			}
			attemptCounter++
			if attemptCounter > s.Cfg.NumNodes*2 {
				break
			}
		}
	}
	log.Printf("Created %d nodes (%d miners), connected peers.\n", s.Cfg.NumNodes, s.Cfg.NumMiners)
}

func (s *Simulation) Run() {
	log.Println("Starting simulation run...")
	s.Setup()

	if s.Cfg.TransactionRatePerSec > 0 && s.Cfg.TotalInputTransactions > 0 {
		firstTxDelay := time.Duration(float64(time.Second) / s.Cfg.TransactionRatePerSec)
		firstTxTime := s.StartTime.Add(firstTxDelay)
		s.ScheduleEvent(firstTxTime, EvInjectTransaction, InjectTransactionData{})
	} else {
		log.Println("Warning: TransactionRatePerSec or TotalInputTransactions is zero, no transactions will be injected.")
	}

	lastProgressLogTime := s.StartTime
	eventCounter := 0
	stopReason := "event queue empty"

	for {

		if s.EventQueue.Len() == 0 {
			stopReason = "event queue empty"
			log.Printf("Simulation stopping at T=%.3fs. Reason: %s.", s.CurrentTime.Sub(s.StartTime).Seconds(), stopReason)
			break
		}

		nextEventTimestamp := s.EventQueue[0].Timestamp
		if nextEventTimestamp.Sub(s.StartTime) >= s.Cfg.SimulationDuration {

			s.CurrentTime = s.StartTime.Add(s.Cfg.SimulationDuration)
			stopReason = fmt.Sprintf("duration limit (%.3fs) reached", s.Cfg.SimulationDuration.Seconds())
			log.Printf("Simulation stopping. Reason: %s.", stopReason)
			break
		}

		event := heap.Pop(&s.EventQueue).(*Event)

		if event.Timestamp.Before(s.CurrentTime) {
			event.Timestamp = s.CurrentTime
		}
		s.CurrentTime = event.Timestamp
		eventCounter++

		if s.CurrentTime.Sub(lastProgressLogTime) > 20*time.Second || eventCounter%10000 == 0 {
			log.Printf("T=%.3fs/%.3fs | Events: %d | Queue: %d | Injected Txs: %d/%d | Stale Blocks(G): %d",
				s.CurrentTime.Sub(s.StartTime).Seconds(), s.Cfg.SimulationDuration.Seconds(),
				eventCounter, s.EventQueue.Len(), s.TxSource.GeneratedCount, s.Cfg.TotalInputTransactions, s.GlobalStaleCount)
			lastProgressLogTime = s.CurrentTime
		}

		switch event.Type {
		case EvInjectTransaction:
			s.handleInjectTransaction()
		case EvReceiveTransaction:
			data := event.Data.(ReceiveTransactionData)
			if node, ok := s.Nodes[data.TargetNodeID]; ok {
				node.ReceiveTransaction(data.Tx)
			}
		case EvAttemptMining:
			data := event.Data.(AttemptMiningData)
			if node, ok := s.Nodes[data.MinerNodeID]; ok {
				node.AttemptMining(data)
			}
		case EvBlockFound:
			data := event.Data.(BlockFoundData)
			if node, ok := s.Nodes[data.MinerNodeID]; ok {
				node.ProcessFoundBlock(data, event)
			}
		case EvReceiveBlock:
			data := event.Data.(ReceiveBlockData)
			if node, ok := s.Nodes[data.TargetNodeID]; ok {
				node.ReceiveBlock(data.Block)
			}
		default:
			log.Printf("Warning: Unknown event type %d encountered\n", event.Type)
		}
	}

	log.Printf("Simulation loop finished. Reason: %s. Final Sim Time: %.3f seconds\n", stopReason, s.CurrentTime.Sub(s.StartTime).Seconds())
}

func (s *Simulation) handleInjectTransaction() {
	if s.TxSource.GeneratedCount >= s.TxSource.TotalToGenerate {
		return
	}
	tx, more := s.TxSource.GetNextTransaction(s.CurrentTime)
	if !more {
		return
	}
	s.AllInputTxHashes[tx.ID] = true
	s.TxStatus[tx.ID] = &TxMetadata{InjectTime: s.CurrentTime}
	originNodeID := rand.Intn(s.Cfg.NumNodes)
	s.ScheduleEventWithPriority(s.CurrentTime, EvReceiveTransaction, ReceiveTransactionData{TargetNodeID: originNodeID, Tx: *tx}, 1)
	if s.TxSource.GeneratedCount < s.TxSource.TotalToGenerate && s.Cfg.TransactionRatePerSec > 0 {
		nextInjectDelay := time.Duration(float64(time.Second) / s.Cfg.TransactionRatePerSec)
		nextInjectTime := s.CurrentTime.Add(nextInjectDelay)
		if nextInjectTime.Sub(s.StartTime) < s.Cfg.SimulationDuration {
			s.ScheduleEvent(nextInjectTime, EvInjectTransaction, InjectTransactionData{})
		}
	}
}
