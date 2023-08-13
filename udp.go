package main

import (
	"fmt"
	"net"
	"os"
	//pk "github.com/Tnze/go-mc/net/packet"
)

func UDPListen() {
	var err error
	server.UDPListener, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(server.Config.UDP.ServerIP), Port: server.Config.UDP.ServerPort})
	if err != nil {
		server.Logger.Error("[UDP] Failed to listen:", err.Error())
		os.Exit(1)
	}

	server.Logger.Info("[UDP] Listening on " + server.Config.UDP.ServerIP + ":" + fmt.Sprint(server.Config.UDP.ServerPort))
}

func HandleUDPRequest(conn *net.UDPConn, ip string, buffer []byte) {
	//fmt.Println(buffer, string(buffer))
}
