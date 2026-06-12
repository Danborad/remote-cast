//go:build linux

package main

import (
	"context"
	"net"
	"syscall"
)

func listenUDP(network string, addr *net.UDPAddr, ifaceIP net.IP, joinSSDP bool) (*net.UDPConn, error) {
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var sockErr error
			err := c.Control(func(fd uintptr) {
				sockErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, 0x0F, 1) // SO_REUSEPORT on Linux.
			})
			if err != nil {
				return err
			}
			return sockErr
		},
	}

	pc, err := lc.ListenPacket(context.Background(), network, addr.String())
	if err != nil {
		return nil, err
	}
	conn := pc.(*net.UDPConn)

	if err := configureMulticast(conn, ifaceIP, joinSSDP); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func configureMulticast(conn *net.UDPConn, ifaceIP net.IP, joinSSDP bool) error {
	ip4 := ifaceIP.To4()
	if ip4 == nil {
		return nil
	}
	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		var addr [4]byte
		copy(addr[:], ip4)
		if err := syscall.SetsockoptInet4Addr(int(fd), syscall.IPPROTO_IP, syscall.IP_MULTICAST_IF, addr); err != nil {
			sockErr = err
			return
		}
		if joinSSDP {
			var multi [4]byte
			copy(multi[:], net.ParseIP(ssdpAddr).To4())
			mreq := syscall.IPMreq{Multiaddr: multi, Interface: addr}
			if err := syscall.SetsockoptIPMreq(int(fd), syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP, &mreq); err != nil {
				sockErr = err
			}
		}
	})
	if err != nil {
		return err
	}
	return sockErr
}
