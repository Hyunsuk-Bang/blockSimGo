package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type Transaction struct {
	ID        string
	Timestamp time.Time
	Data      string
	Size      int
}

type BlockHeader struct {
	Height    int
	Timestamp time.Time
	PrevHash  string
	MinerID   int
	NumTx     int
}

type Block struct {
	Header       BlockHeader
	Transactions []Transaction
	Hash         string
	FoundTime    time.Time
}

func (b *Block) CalculateHash() string {
	headerStr := fmt.Sprintf("%d%s%s%d%d", b.Header.Height, b.Header.Timestamp.String(), b.Header.PrevHash, b.Header.MinerID, b.Header.NumTx)

	var txIDs []string
	for _, tx := range b.Transactions {
		txIDs = append(txIDs, tx.ID)
	}
	txStr := strings.Join(txIDs, "")
	hashBytes := sha256.Sum256([]byte(headerStr + txStr))
	return hex.EncodeToString(hashBytes[:])
}

func NewBlock(height int, prevHash string, attemptTime time.Time, minerID int, txs []Transaction) Block {
	b := Block{
		Header: BlockHeader{
			Height: height, Timestamp: attemptTime, PrevHash: prevHash, MinerID: minerID, NumTx: len(txs),
		},
		Transactions: txs,
	}
	b.Hash = b.CalculateHash()
	return b
}
