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

func Reload() chat.Message {
	playerCache = make(map[string]PlayerPermissions)
	groupCache = make(map[string]GroupPermissions)
	server.Config = LoadConfig()
	server.Whitelist = LoadPlayerList("whitelist.json")
	server.OPs = LoadPlayerList("ops.json")
	server.BannedPlayers = LoadPlayerList("banned_players.json")
	server.BannedIPs = LoadIPBans()
	server.Favicon = []byte{}
	if server.Config.Whitelist.Enable && server.Config.Whitelist.Enforce {
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
	for _, player := range server.Players {
		player.Connection.WritePacket(pk.Marshal(packetid.ClientboundCommands, CommandGraph{player.UUID.String}))
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

func GetArgument(args []string, index int) string {
	if len(args) <= index {
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
	command, exists := server.Commands[cmd]
	if !exists {
		return chat.Text(server.Config.Messages.UnknownCommand)
	}
	if !server.HasPermissions(executor, command.RequiredPermissions) {
		return chat.Text(server.Config.Messages.InsufficientPermissions)
	}
	switch cmd {
	case "reload":
		return Reload()
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
				player.UUID = id
				if server.Players[id].UUID.String == id {
					player.Name = server.Players[id].Name
				} else {
					exists, p := server.Mojang.FetchUUID(id)
					if !exists {
						if server.Config.Online {
							return chat.Text("§cUnknown player")
						}
						player.Name = id
					} else {
						player.Name = p.Name
					}
				}
			} else {
				player.Name = id
				if server.PlayerNames[id] != "" {
					player.UUID = server.PlayerNames[id]
				} else {
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
			}
			server.OPs = WritePlayerList("ops.json", player)
			if server.Players[player.UUID].UUID.String == player.UUID {
				server.Players[player.UUID].Connection.WritePacket(pk.Marshal(packetid.ClientboundCommands, CommandGraph{player.UUID}))
			}
			server.BroadcastMessageAdmin(executor, chat.Text(fmt.Sprintf("§7[%s: Made %s a server operator]", executorName, player.Name)))
			return chat.Text(fmt.Sprintf("Made %s a server operator", player.Name))
		}
	case "gamemode":
		{
			gamemode := GetArgument(args, 0)
			id := GetArgument(args, 1)
			var player *Player
			if gamemode == "" {
				return chat.Text("§cPlease specify a gamemode")
			}
			if id == "" {
				if executor == "console" {
					return chat.Text("§cThe gamemode command can only be used on players")
				} else {
					id = executor
				}
			}
			var mode float32
			switch gamemode {
			case "survival":
				mode = 0
			case "creative":
				mode = 1
			case "adventure":
				mode = 2
			case "spectator":
				mode = 3
			}
			if server.PlayerNames[id] == "" {
				if server.Players[id].UUID.String != id {
					return chat.Text("§cUnknown player")
				} else {
					player = server.Players[id]
				}
			} else {
				player = server.Players[server.PlayerNames[id]]
			}
			player.Connection.WritePacket(pk.Marshal(
				packetid.ClientboundGameEvent,
				pk.UnsignedByte(3),
				pk.Float(mode),
			))
			server.BroadcastMessageAdmin(executor, chat.Text(fmt.Sprintf("§7[%s: Set %s's gamemode to %s]", executorName, player.Name, gamemode)))
			if executor == player.UUID.String {
				return chat.Text(fmt.Sprintf("Set own gamemode to %s", gamemode))
			} else {
				return chat.Text(fmt.Sprintf("Set %s's gamemode to %s", player.Name, gamemode))
			}
		}
	default:
		return chat.Text(server.Config.Messages.UnknownCommand)
	}
}
