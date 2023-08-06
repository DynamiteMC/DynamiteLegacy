package main

import (
	"fmt"
	"net"
	"os"
	"time"

	mcnet "github.com/Tnze/go-mc/net"
)

var server = Server{}
var logger = Logger{}
var startTime int64
var config = LoadConfig()
var TCPListener *mcnet.Listener
var UDPListener *net.UDPConn

type Server struct {
	Players []Player
}

func main() {
	startTime = time.Now().Unix()
	logger.Info("Starting GoCraft")
	//if !HasArg("-nogui") {
	//	logger.Info("Launching GUI panel. Disable this using -nogui")
	//	//go LaunchGUI()
	//}
	LoadPlayerList("whitelist.json")
	LoadPlayerList("ops.json")
	LoadPlayerList("banned_players.json")
	LoadIPBans()
	logger.Debug("Loaded player info")
	if !config.Online && !HasArg("-no_offline_warn") {
		logger.Warn("Offline mode is insecure. You can disable this message using -no_offline_warn")
	}
	if config.TCP.Enable {
		var err error
		TCPListener, err = mcnet.ListenMC(config.TCP.ServerIP + ":" + fmt.Sprint(config.TCP.ServerPort))
		if err != nil {
			logger.Error("[TCP] Failed to listen:", err.Error())
			os.Exit(1)
		}
		defer TCPListener.Close()

		logger.Info("[TCP] Listening on " + config.TCP.ServerIP + ":" + fmt.Sprint(config.TCP.ServerPort))
	}
	if config.UDP.Enable {
		var err error
		UDPListener, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(config.UDP.ServerIP), Port: config.UDP.ServerPort})
		if err != nil {
			logger.Error("[UDP] Failed to listen:", err.Error())
			os.Exit(1)
		}
		defer UDPListener.Close()

		logger.Info("[UDP] Listening on " + config.UDP.ServerIP + ":" + fmt.Sprint(config.UDP.ServerPort))
	}

	logger.Info("Done!", "("+fmt.Sprint(time.Now().Unix()-startTime)+"s)")
	go CreateSTDINReader()
	for {
		if config.TCP.Enable {
			conn, err := TCPListener.Accept()
			if err != nil {
				fmt.Println("Error accepting: ", err.Error())
				os.Exit(1)
			}
			go handleTCPRequest(conn)
		}
	}
}
