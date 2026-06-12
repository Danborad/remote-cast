//go:build !linux

package main

import "net"

func listenUDP(network string, addr *net.UDPAddr, ifaceIP net.IP, joinSSDP bool) (*net.UDPConn, error) {
	return net.ListenUDP(network, addr)
}
