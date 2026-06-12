//go:build !linux

package main

import (
	"os"
	"os/signal"
)

func notifySignals(ch chan<- os.Signal) {
	signal.Notify(ch, os.Interrupt)
}

func isScanSignal(sig os.Signal) bool {
	return false
}
