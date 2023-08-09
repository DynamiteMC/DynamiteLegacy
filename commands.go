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
			server.Logger.Info("["+player.IP+"]", "Player", player.Name, "("+player.UUID+")", "disconnected")
		}
	}
	return chat.Text("Reloaded config successfully")
}

func CreateSTDINReader() {
	reader := bufio.NewReader(os.Stdin)
	command, _ := reader.ReadString('\n')
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
