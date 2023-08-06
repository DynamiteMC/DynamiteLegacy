package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
)

const (
	CONN_HOST = "0.0.0.0"
	CONN_PORT = "3333"
)

var logger = Logger{}
var startTime = time.Now().Unix()

func main() {
	logger.Info("Starting GoCraft")
	if !HasArg("-nogui") {
		logger.Info("Launching GUI panel. Disable this using -nogui")
		//go LaunchGUI()
	}
	l, err := net.ListenMC(CONN_HOST + ":" + CONN_PORT)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	defer l.Close()
	logger.Info("Listening on " + CONN_HOST + ":" + CONN_PORT)
	logger.Info("Done!", "("+fmt.Sprint(time.Now().Unix()-startTime)+"s)")
	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		go handleRequest(conn)
	}
}

func handleRequest(conn net.Conn) {
	defer conn.Close()
	var packet pk.Packet
	conn.ReadPacket(&packet)
	ip := conn.Socket.RemoteAddr().String()
	if packet.ID == 0x00 { // 1.7+
		logger.Debug("(["+ip+"]", "-> Server) Sent handshake")
		var (
			Protocol, Intention pk.VarInt
			ServerAddress       pk.String
			ServerPort          pk.UnsignedShort
		)
		err := packet.Scan(&Protocol, &ServerAddress, &ServerPort, &Intention)
		if err != nil {
			return
		}

		switch Intention {
		case 1: // Ping
			{
				var p pk.Packet
				for i := 0; i < 2; i++ {
					conn.ReadPacket(&p)
					switch p.ID {
					case packetid.StatusRequest:
						logger.Debug("(["+ip+"]", "-> Server) Sent StatusRequest packet")
						resp, _ := os.ReadFile("response.txt")
						conn.WritePacket(pk.Marshal(0x00, pk.String(resp)))
						logger.Debug("(Server -> ["+ip+"])", "Sent StatusResponse packet")
					case packetid.StatusPingRequest:
						logger.Debug("(["+ip+"]", "-> Server) Sent StatusPingRequest packet")
						conn.WritePacket(p)
						logger.Debug("(Server -> ["+ip+"])", "Sent StatusPongResponse packet")
					}
				}
			}
		case 2:
			{ // login
				var p pk.Packet
				conn.ReadPacket(&p)
				var (
					name pk.String
					uuid pk.UUID
				)
				p.Scan(&name, &uuid)
				logger.Debug("(["+ip+"]", "-> Server) Sent LoginStart packet. Username:", name)
				logger.Info("["+ip+"]", "Player", name, "is attempting to join.")
				fmt.Println(uuid)
			}
		}
	} else if packet.ID == 122 { //1.6-
	}
}
