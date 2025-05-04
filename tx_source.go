package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"
)

type SimpleTxSource struct {
	TotalToGenerate int
	GeneratedCount  int
	StartTime       time.Time
	Cfg             *Config
}

func NewSimpleTxSource(cfg *Config, startTime time.Time) *SimpleTxSource {
	return &SimpleTxSource{
		TotalToGenerate: cfg.TotalInputTransactions,
		StartTime:       startTime,
		GeneratedCount:  0,
		Cfg:             cfg,
	}
}

func (s *SimpleTxSource) GetNextTransaction(currentTime time.Time) (*Transaction, bool) {
	if s.GeneratedCount >= s.TotalToGenerate {
		return nil, false
	}
	s.GeneratedCount++

	mean := s.Cfg.MeanTransactionSizeBytes
	stdDev := s.Cfg.StdDevTransactionSizeBytes
	minClamp := float64(s.Cfg.MinTransactionSizeBytes)
	maxClamp := float64(s.Cfg.MaxTransactionSizeBytes)

	var sizeFloat float64
	if stdDev > 0 {
		sizeFloat = rand.NormFloat64()*stdDev + mean
	} else {
		sizeFloat = mean
	}

	sizeFloat = math.Max(minClamp, sizeFloat)
	sizeFloat = math.Min(maxClamp, sizeFloat)

	size := int(math.Round(sizeFloat))
	tx := Transaction{
		ID:        fmt.Sprintf("tx-%d-%d", s.GeneratedCount, rand.Intn(1000000)),
		Timestamp: currentTime,
		Data:      "simulated payload data",
		Size:      size,
	}
	return &tx, true
}
