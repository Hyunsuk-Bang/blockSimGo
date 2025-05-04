package main

import (
	"time"
)

type EventType int

const (
	EvInjectTransaction EventType = iota
	EvReceiveTransaction
	EvAttemptMining
	EvBlockFound
	EvReceiveBlock
)

type Event struct {
	Timestamp time.Time
	Type      EventType
	Data      interface{}
	Priority  int
	index     int
}

type InjectTransactionData struct{}
type ReceiveTransactionData struct {
	TargetNodeID int
	Tx           Transaction
}
type AttemptMiningData struct {
	MinerNodeID     int
	ParentBlockHash string
	Height          int
}
type BlockFoundData struct {
	MinerNodeID int
	Block       Block
}
type ReceiveBlockData struct {
	TargetNodeID int
	Block        Block
}

type EventQueue []*Event

func (eq EventQueue) Len() int { return len(eq) }

func (eq EventQueue) Less(i, j int) bool {
	if eq[i].Timestamp.Before(eq[j].Timestamp) {
		return true
	}
	if eq[i].Timestamp.After(eq[j].Timestamp) {
		return false
	}

	return eq[i].Priority < eq[j].Priority
}

func (eq EventQueue) Swap(i, j int) {
	eq[i], eq[j] = eq[j], eq[i]
	eq[i].index = i
	eq[j].index = j
}

func (eq *EventQueue) Push(x interface{}) {
	n := len(*eq)
	event := x.(*Event)
	event.index = n
	*eq = append(*eq, event)
}

func (eq *EventQueue) Pop() interface{} {
	old := *eq
	n := len(old)
	event := old[n-1]
	old[n-1] = nil
	event.index = -1
	*eq = old[0 : n-1]
	return event
}
