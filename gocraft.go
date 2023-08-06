package main

import (
	"fmt"
	"os"
	"time"

	"github.com/Tnze/go-mc/net"
)

var server = Server{}
var logger = Logger{}
var startTime int64
var config = LoadConfig()
var TCPListener *net.Listener

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
		TCPListener, err = net.ListenMC(config.TCP.ServerIP + ":" + fmt.Sprint(config.TCP.ServerPort))
		if err != nil {
			logger.Error("[TCP] Failed to listen:", err.Error())
			os.Exit(1)
		}
		defer TCPListener.Close()

		logger.Info("[TCP] Listening on " + config.TCP.ServerIP + ":" + fmt.Sprint(config.TCP.ServerPort))
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
