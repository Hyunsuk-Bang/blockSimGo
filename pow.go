package main

import (
	"math/rand"
	"time"
)

func CalculateTimeToFind(cfg *Config) time.Duration {
	minDuration := cfg.FindTimeMin
	maxDuration := cfg.FindTimeMax
	minSeconds := float64(minDuration.Seconds())
	maxSeconds := float64(maxDuration.Seconds())
	rangeSeconds := maxSeconds - minSeconds
	if rangeSeconds <= 0 {
		return minDuration
	}
	randomSecondsInAddition := rand.Float64() * rangeSeconds
	totalSeconds := minSeconds + randomSecondsInAddition
	calculatedDuration := time.Duration(totalSeconds * float64(time.Second))
	return calculatedDuration
}
