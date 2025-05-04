package main

import (
	"math/rand"
	"time"
)

func contains(slice []int, item int) bool {
	for _, a := range slice {
		if a == item {
			return true
		}
	}
	return false
}

func CalculateNetworkDelay(cfg *Config) time.Duration {
	minDelay := float64(cfg.NetworkDelayMin)
	maxDelay := float64(cfg.NetworkDelayMax)

	if minDelay >= maxDelay {
		return cfg.NetworkDelayMin
	}
	delay := minDelay + rand.Float64()*(maxDelay-minDelay)
	return time.Duration(delay)
}
