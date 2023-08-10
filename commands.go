package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	pk "github.com/Tnze/go-mc/net/packet"
)

func ReloadConfig() chat.Message {
	playerCache = make(map[string]PlayerPermissions)
	groupCache = make(map[string]GroupPermissions)
	newConfig := LoadConfig()
	if newConfig.Whitelist.Enable && newConfig.Whitelist.Enforce {
		whitelist := LoadPlayerList("whitelist.json")
		wmap := make(map[string]bool)
		for _, p := range whitelist {
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
	server.Logger.Print(server.Command("console", command))
	go CreateSTDINReader()
}

var Commands = map[string]Command{
	"stop": {
		Name:                "stop",
		RequiredPermissions: []string{"server.stop"},
		Executable:          true,
	},
	"reload": {
		Name:                "reload",
		RequiredPermissions: []string{"server.reload"},
		Executable:          true,
	},
	"op": {
		Name:                "op",
		RequiredPermissions: []string{"server.op"},
		Executable:          false,
		Arguments: []Argument{
			{
				Name:            "player",
				SuggestionsType: "minecraft:ask_server",
				ParserID:        "minecraft:entity",
			},
		},
	},
}

func (server Server) Command(executor string, cmd string) chat.Message {
	cmd = strings.TrimSpace(cmd)
	command := Commands[cmd]
	if !server.HasPermissions(executor, command.RequiredPermissions) {
		return chat.Text(server.Config.Messages.InsufficientPermissions)
	}
	switch cmd {
	case "reload":
		{
			return ReloadConfig()
		}
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
		{
			return chat.Text(server.Config.Messages.UnknownCommand)
		}
	}
}
