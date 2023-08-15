package main

import (
	"embed"
	"fmt"
	"os"
	"os/signal"
	"time"

	pk "github.com/Tnze/go-mc/net/packet"
)

var server = Server{
	Config:      LoadConfig(),
	Players:     make(map[string]Player),
	PlayerNames: make(map[string]string),
	PlayerIDs:   make([]string, 0),
	Events:      Events{_Events: make(map[string][]func(...interface{}))},
	Commands: map[string]Command{
		"gamemode": {
			Name:                "gamemode",
			RequiredPermissions: []string{"server.command.gamemode"},
			Arguments: []Argument{
				{
					Name: "mode",
					Parser: Parser{
						ID:   39,
						Name: "minecraft:gamemode",
					},
				},
				{
					Name: "player",
					Parser: Parser{
						ID:         6,
						Name:       "minecraft:entity",
						Properties: pk.Byte(0x02),
					},
				},
			},
		},
		"op": {
			Name:                "op",
			RequiredPermissions: []string{"server.command.op"},
			Arguments: []Argument{
				{
					Name: "player",
					Parser: Parser{
						ID:         6,
						Name:       "minecraft:entity",
						Properties: pk.Byte(0x02),
					},
				},
			},
		},
		"reload": {
			Name:                "reload",
			RequiredPermissions: []string{"server.command.reload"},
		},
		"stop": {
			Name:                "stop",
			RequiredPermissions: []string{"server.command.stop"},
		},
	},
}

//go:embed registry
var registries embed.FS

func main() {
	server.StartTime = time.Now().Unix()
	server.Logger.Info("Starting GoCraft")
	server.Init()
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
					conn, err := server.TCPListener.Accept()
					if err != nil {
						continue
					}
					go HandleTCPRequest(conn)
				}
			}
		}()
		server.Logger.Info("Launching GUI panel")
		server.Logger.Info("Done! (%d)", time.Now().Unix()-server.StartTime)
		LaunchGUI().ShowAndRun()
	} else {
		server.Logger.Info("Done! (%d)", time.Now().Unix()-server.StartTime)
		for {
			if server.Config.TCP.Enable {
				conn, err := server.TCPListener.Accept()
				if err != nil {
					continue
				}
				go HandleTCPRequest(conn)
			}
			if server.Config.UDP.Enable {
				buffer := make([]byte, 1024)
				_, ip, err := server.UDPListener.ReadFromUDP(buffer)
				if err != nil {
					continue
				}
				go HandleUDPRequest(server.UDPListener, ip.String(), buffer)
				continue
			}
		}
	}
}
