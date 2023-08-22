package main

import (
	"embed"
	"fmt"
	"gocraft/logger"
	"os"
	"os/signal"
	"time"

	pk "github.com/Tnze/go-mc/net/packet"
)

var server = Server{
	Config:      LoadConfig(),
	Players:     make(map[string]*Player),
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
		"teleport": {
			Name:                "teleport",
			RequiredPermissions: []string{"server.command.teleport"},
			Aliases:             []string{"tp"},
			Arguments: []Argument{
				{
					Name: "one",
					Parser: Parser{
						ID:   8,
						Name: "minecraft:block_pos",
					},
				},
				{
					Name: "two",
					Parser: Parser{
						ID:   8,
						Name: "minecraft:block_pos",
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
			Aliases:             []string{"rl"},
		},
		"stop": {
			Name:                "stop",
			RequiredPermissions: []string{"server.command.stop"},
		},
		"ram": {
			Name:                "ram",
			RequiredPermissions: []string{},
		},
	},
}

//go:embed registry.nbt
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
	if logger.HasArg("-gui") {
		go func() {
			for {
				conn, err := server.Listener.Accept()
				if err != nil {
					continue
				}
				go HandleTCPRequest(conn)
			}
		}()
		server.Logger.Info("Launching GUI panel")
		server.Logger.Info("Done! (%ds)", time.Now().Unix()-server.StartTime)
		LaunchGUI().ShowAndRun()
	} else {
		server.Logger.Info("Done! (%ds)", time.Now().Unix()-server.StartTime)
		for {
			conn, err := server.Listener.Accept()
			if err != nil {
				continue
			}
			go HandleTCPRequest(conn)
		}
	}
}
