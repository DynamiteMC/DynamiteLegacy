package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	mcnet "github.com/Tnze/go-mc/net"
)

var TCPListener *mcnet.Listener
var UDPListener *net.UDPConn

var server = Server{
	Config:  LoadConfig(),
	Players: make(map[string]Player),
	Events:  Events{_events: make(map[string][]func(...interface{}))},
	Logger:  Logger{},
}

func main() {
	server.StartTime = time.Now().Unix()
	server.Logger.Info("Starting GoCraft")
	LoadPlayerList("whitelist.json")
	LoadPlayerList("ops.json")
	LoadPlayerList("banned_players.json")
	os.Mkdir("permissions", 0755)
	LoadIPBans()
	server.Logger.Debug("Loaded player info")
	if !server.Config.Online && !HasArg("-no_offline_warn") {
		server.Logger.Warn("Offline mode is insecure. You can disable this message using -no_offline_warn")
	}
	if server.Config.TCP.Enable {
		var err error
		TCPListener, err = mcnet.ListenMC(server.Config.TCP.ServerIP + ":" + fmt.Sprint(server.Config.TCP.ServerPort))
		if err != nil {
			server.Logger.Error("[TCP] Failed to listen:", err.Error())
			os.Exit(1)
		}
		defer TCPListener.Close()

		server.Logger.Info("[TCP] Listening on " + server.Config.TCP.ServerIP + ":" + fmt.Sprint(server.Config.TCP.ServerPort))
	}
	if server.Config.UDP.Enable {
		var err error
		UDPListener, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(server.Config.UDP.ServerIP), Port: server.Config.UDP.ServerPort})
		if err != nil {
			server.Logger.Error("[UDP] Failed to listen:", err.Error())
			os.Exit(1)
		}
		defer UDPListener.Close()

		server.Logger.Info("[UDP] Listening on " + server.Config.UDP.ServerIP + ":" + fmt.Sprint(server.Config.UDP.ServerPort))
	}
	CreateEvents()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			fmt.Println(server.Command("console", "stop"))
		}
	}()
	go CreateSTDINReader()
	if HasArg("-gui") {
		go func() {
			for {
				if server.Config.TCP.Enable {
					conn, err := TCPListener.Accept()
					if err != nil {
						fmt.Println("Error accepting: ", err.Error())
						os.Exit(1)
					}
					go HandleTCPRequest(conn)
				}
			}
		}()
		server.Logger.Info("Launching GUI panel")
		server.Logger.Info("Done!", "("+fmt.Sprint(time.Now().Unix()-server.StartTime)+"s)")
		LaunchGUI().ShowAndRun()
	} else {
		server.Logger.Info("Done!", "("+fmt.Sprint(time.Now().Unix()-server.StartTime)+"s)")
		for {
			if server.Config.TCP.Enable {
				conn, err := TCPListener.Accept()
				if err != nil {
					fmt.Println("Error accepting: ", err.Error())
					os.Exit(1)
				}
				go HandleTCPRequest(conn)
			}
			if server.Config.UDP.Enable {
				buffer := make([]byte, 1024)
				_, ip, err := UDPListener.ReadFromUDP(buffer)
				if err != nil {
					continue
				}
				go handleUDPRequest(UDPListener, ip.String(), buffer)
				continue
			}
		}
	}
}
