package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	pk "github.com/Tnze/go-mc/net/packet"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func ReloadConfig() chat.Message {
	playerCache = make(map[string]PlayerPermissions)
	groupCache = make(map[string]GroupPermissions)
	newConfig := LoadConfig()
	server.Whitelist = LoadPlayerList("whitelist.json")
	server.OPs = LoadPlayerList("ops.json")
	server.BannedPlayers = LoadPlayerList("banned_players.json")
	server.BannedIPs = LoadIPBans()
	server.Favicon = []byte{}
	if newConfig.Whitelist.Enable && newConfig.Whitelist.Enforce {
		wmap := make(map[string]bool)
		for _, p := range server.Whitelist {
			wmap[p.UUID] = true
		}
		for i := 0; i < len(server.Players) && !wmap[server.PlayerIDs[i]]; i++ {
			player := server.Players[server.PlayerIDs[i]]
			player.Connection.WritePacket(pk.Marshal(packetid.ClientboundDisconnect, chat.Text(server.Config.Messages.NotInWhitelist)))
			player.Connection.Close()
			server.Events.Emit("PlayerLeave", player)
		}
	}
	return chat.Text(server.Config.Messages.ReloadComplete)
}

func CreateSTDINReader() {
	reader := bufio.NewReader(os.Stdin)
	command, _ := reader.ReadString('\n')
	command = strings.TrimSpace(command)
	if command == "" {
		go CreateSTDINReader()
		return
	}
	server.Logger.Print("%v", server.Command("console", command))
	go CreateSTDINReader()
}

var Commands = orderedmap.New[string, Command](orderedmap.WithInitialData[string, Command](
	orderedmap.Pair[string, Command]{
		Key: "op",
		Value: Command{
			Name:                "op",
			RequiredPermissions: []string{"server.op"},
			Arguments: []Argument{
				{
					Name:            "player",
					SuggestionsType: "minecraft:ask_server",
					ParserID:        6,
				},
			},
		},
	},
	orderedmap.Pair[string, Command]{
		Key: "reload",
		Value: Command{
			Name:                "reload",
			RequiredPermissions: []string{"server.reload"},
			Aliases:             []string{"rl"},
		},
	},
	orderedmap.Pair[string, Command]{
		Key: "stop",
		Value: Command{
			Name:                "stop",
			RequiredPermissions: []string{"server.stop"},
		},
	},
))

func (server Server) Command(executor string, cmd string) chat.Message {
	cmd = strings.TrimSpace(cmd)
	command, exists := Commands.Get(cmd)
	if !exists {
		return chat.Text(server.Config.Messages.UnknownCommand)
	}
	if !server.HasPermissions(executor, command.RequiredPermissions) {
		return chat.Text(server.Config.Messages.InsufficientPermissions)
	}
	switch cmd {
	case "reload":
		return ReloadConfig()
	case "stop":
		{
			go func() {
				for _, player := range server.Players {
					player.Connection.WritePacket(pk.Marshal(packetid.ClientboundDisconnect, chat.Text(server.Config.Messages.ServerClosed)))
					player.Connection.Close()
				}
				os.Exit(0)
			}()
			return chat.Text("Shutting down server...")
		}
	default:
		return chat.Text(server.Config.Messages.UnknownCommand)
	}
}
