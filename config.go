package main

import "time"

type Config struct {
	NumNodes              int
	NumMiners             int
	BlockSizeLimitBytes   int
	TargetBlockInterval   time.Duration
	TransactionRatePerSec float64

	MinTransactionSizeBytes    int     `default:"100"`
	MaxTransactionSizeBytes    int     `default:"600"`
	MeanTransactionSizeBytes   float64 `default:"130.0"`
	StdDevTransactionSizeBytes float64 `default:"150.0"`

	NetworkDelayMin        time.Duration
	NetworkDelayMax        time.Duration
	TotalInputTransactions int
	SimulationDuration     time.Duration

	FindTimeMin time.Duration `default:"9m"`
	FindTimeMax time.Duration `default:"11m"`
}

func DefaultConfig() Config {
	return Config{
		NumNodes:              20,
		NumMiners:             5,
		BlockSizeLimitBytes:   1 * 1024 * 1024,
		TargetBlockInterval:   10 * time.Minute,
		TransactionRatePerSec: 4.0,

		MinTransactionSizeBytes:    100,
		MaxTransactionSizeBytes:    600,
		MeanTransactionSizeBytes:   300.0,
		StdDevTransactionSizeBytes: 150.0,

		NetworkDelayMin:        100 * time.Millisecond,
		NetworkDelayMax:        500 * time.Millisecond,
		TotalInputTransactions: 20000,
		SimulationDuration:     1 * time.Hour,

		FindTimeMin: 10 * time.Minute,
		FindTimeMax: 11 * time.Minute,
	}
}
