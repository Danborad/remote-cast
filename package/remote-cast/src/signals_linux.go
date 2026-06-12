//go:build linux

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func notifySignals(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGUSR1)
}

func isScanSignal(sig os.Signal) bool {
	return sig == syscall.SIGUSR1
}
