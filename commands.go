package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/offline"
	"github.com/google/uuid"
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

var Commands = map[string]Command{
	"op": {
		Name:                "op",
		RequiredPermissions: []string{"server.op"},
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
		RequiredPermissions: []string{"server.reload"},
	},
	"stop": {
		Name:                "stop",
		RequiredPermissions: []string{"server.stop"},
	},
}

func GetArgument(args []string, index int) string {
	if len(args) < index {
		return ""
	}
	return args[index]
}

func (server *Server) Command(executor string, content string) chat.Message {
	var executorName string
	if executor == "console" {
		executorName = "Console"
	} else {
		executorName = server.Players[executor].Name
	}
	content = strings.TrimSpace(content)
	args := strings.Split(content, " ")
	cmd := args[0]
	args = args[1:]
	command, exists := Commands[cmd]
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
	case "op":
		{
			id := GetArgument(args, 0)
			if id == "" {
				return chat.Text("§cPlease specify a player to op")
			}
			isOp, op := server.IsOP(id)
			if isOp {
				return chat.Text(fmt.Sprintf("§c%s is already a server operator", op.Name))
			}
			player := PlayerBase{}
			if _, err := uuid.Parse(id); err == nil { // is uuid
				exists, p := server.Mojang.FetchUUID(id)
				if !exists {
					if server.Config.Online {
						return chat.Text("§cUnknown player")
					}
					player.Name = id
				} else {
					player.Name = p.Name
				}
				player.UUID = id
			} else {
				player.Name = id
				exists, p := server.Mojang.FetchUsername(id)
				if !exists {
					if server.Config.Online {
						return chat.Text("§cUnknown player")
					} else {
						player.UUID = fmt.Sprint(offline.NameToUUID(id))
					}
				} else {
					player.UUID = p.UUID
				}
			}
			server.OPs = WritePlayerList("ops.json", player)
			server.BroadcastMessageAdmin(executor, chat.Text(fmt.Sprintf("§7[%s: Made %s a server operator]", executorName, player.Name)))
			return chat.Text(fmt.Sprintf("Made %s a server operator", player.Name))
		}
	default:
		return chat.Text(server.Config.Messages.UnknownCommand)
	}
}
