package main

import (
	"fmt"
	"net"
)

func handleUDPRequest(conn *net.UDPConn) {
	buf := make([]byte, 1024)
	_, r, _ := conn.ReadFromUDP(buf)
	fmt.Println(string(buf), r)
}
