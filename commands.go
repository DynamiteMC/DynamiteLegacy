package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
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
	server.Players.Whitelist = LoadPlayerList("whitelist.json")
	server.Players.OPs = LoadPlayerList("ops.json")
	server.Players.BannedPlayers = LoadPlayerList("banned_players.json")
	server.Players.BannedIPs = LoadIPBans()
	server.Favicon = []byte{}
	if server.Config.Whitelist.Enable && server.Config.Whitelist.Enforce {
		wmap := make(map[string]bool)
		for _, p := range server.Players.Whitelist {
			wmap[p.UUID] = true
		}
		for i := 0; i < len(server.Players.Players) && !wmap[server.Players.PlayerIDs[i]]; i++ {
			player := server.Players.Players[server.Players.PlayerIDs[i]]
			player.Connection.WritePacket(pk.Marshal(packetid.ClientboundDisconnect, chat.Text(server.Config.Messages.NotInWhitelist)))
			player.Connection.Close()
			server.Events.Emit("PlayerLeave", player)
		}
	}
	for _, player := range server.Players.Players {
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
	var executorPlayer *Player
	if executor == "console" {
		executorName = "Console"
	} else {
		executorName = server.Players.Players[executor].Name
		executorPlayer = server.Players.Players[executor]
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
	case "reload", "rl":
		return Reload()
	case "stop":
		{
			go func() {
				server.Logger.Info("Saving world")
				server.Players.Lock()
				for _, player := range server.Players.Players {
					player.Connection.WritePacket(pk.Marshal(packetid.ClientboundDisconnect, chat.Text(server.Config.Messages.ServerClosed)))
					player.Connection.Close()
					player.Data.Save(player.UUID.String)
				}
				for _, world := range server.Worlds {
					world.TickLock.Lock()
					for pos, chunk := range world.Chunks {
						chunk.Lock()
						world.UnloadChunk(pos)
					}
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
			isOp, op := server.Players.IsOP(id)
			if isOp {
				return chat.Text(fmt.Sprintf("§c%s is already a server operator", op.Name))
			}
			player := PlayerBase{}
			if _, err := uuid.Parse(id); err == nil { // is uuid
				player.UUID = id
				if server.Players.Players[id].UUID.String == id {
					player.Name = server.Players.Players[id].Name
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
				if server.Players.PlayerNames[id] != "" {
					player.UUID = server.Players.PlayerNames[id]
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
			server.Players.OPs = WritePlayerList("ops.json", player)
			if server.Players.Players[player.UUID] != nil {
				server.Players.Players[player.UUID].Connection.WritePacket(pk.Marshal(packetid.ClientboundCommands, CommandGraph{player.UUID}))
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
			if server.Players.PlayerNames[id] == "" {
				if server.Players.Players[id].UUID.String != id {
					return chat.Text("§cUnknown player")
				} else {
					player = server.Players.Players[id]
				}
			} else {
				player = server.Players.Players[server.Players.PlayerNames[id]]
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
	case "ram":
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return chat.Text(fmt.Sprintf("Allocated: %v MiB, Total Allocated: %v MiB", bToMb(m.Alloc), bToMb(m.TotalAlloc)))
	case "teleport", "tp":
		{
			switch len(args) {
			case 1:
				{
					if executor == "console" {
						return chat.Text("§cOnly players can be teleported")
					}
					is, target := ParseTarget(executorPlayer, args[0])
					if !is {
						return chat.Text("§cUnknown player at argument 0")
					}
					executorPlayer.Connection.WritePacket(pk.Marshal(packetid.ClientboundPlayerPosition,
						pk.Double(target.Position[0]),        //x
						pk.Double(target.Position[1]),        //y
						pk.Double(target.Position[2]),        //z
						pk.Float(executorPlayer.Rotation[0]), //yaw
						pk.Float(executorPlayer.Rotation[1]), //pitch
						pk.Byte(0),
						pk.VarInt(server.NewTeleportID()),
					))
					return chat.Text(fmt.Sprintf("Teleported you to %s", target.Name))
				}
			case 2:
				{
					is, player := ParseTarget(executorPlayer, args[0])
					if !is {
						return chat.Text("§cUnknown player at argument 0")
					}
					is, target := ParseTarget(executorPlayer, args[1])
					if !is {
						return chat.Text("§cUnknown player at argument 1")
					}
					player.Connection.WritePacket(pk.Marshal(packetid.ClientboundPlayerPosition,
						pk.Double(target.Position[0]), //x
						pk.Double(target.Position[1]), //y
						pk.Double(target.Position[2]), //z
						pk.Float(player.Rotation[0]),  //yaw
						pk.Float(player.Rotation[1]),  //pitch
						pk.Byte(0),
						pk.VarInt(server.NewTeleportID()),
					))
					return chat.Text(fmt.Sprintf("Teleported %s to %s", player.Name, target.Name))
				}
			case 3:
				{
					x, xerr := strconv.ParseFloat(args[0], 64)
					y, yerr := strconv.ParseFloat(args[1], 64)
					z, zerr := strconv.ParseFloat(args[2], 64)
					if xerr != nil || yerr != nil || zerr != nil {
						return chat.Text("§cPassed argument is not a float")
					}
					executorPlayer.Connection.WritePacket(pk.Marshal(packetid.ClientboundPlayerPosition,
						pk.Double(x),                         //x
						pk.Double(y),                         //y
						pk.Double(z),                         //z
						pk.Float(executorPlayer.Rotation[0]), //yaw
						pk.Float(executorPlayer.Rotation[1]), //pitch
						pk.Byte(0),
						pk.VarInt(server.NewTeleportID()),
					))
					return chat.Text(fmt.Sprintf("Teleported you to %v %v %v", x, y, z))
				}
			case 4:
				{
					is, player := ParseTarget(executorPlayer, args[0])
					if !is {
						return chat.Text("§cUnknown player at argument 0")
					}
					x, xerr := strconv.ParseFloat(args[1], 64)
					y, yerr := strconv.ParseFloat(args[2], 64)
					z, zerr := strconv.ParseFloat(args[3], 64)
					if xerr != nil || yerr != nil || zerr != nil {
						return chat.Text("§cPassed argument is not a float")
					}
					player.Connection.WritePacket(pk.Marshal(packetid.ClientboundPlayerPosition,
						pk.Double(x),                 //x
						pk.Double(y),                 //y
						pk.Double(z),                 //z
						pk.Float(player.Rotation[0]), //yaw
						pk.Float(player.Rotation[1]), //pitch
						pk.Byte(0),
						pk.VarInt(server.NewTeleportID()),
					))
					return chat.Text(fmt.Sprintf("Teleported %s to %v %v %v", player.Name, x, y, z))
				}
			default:
				return chat.Text("§cInvalid amount of arguments")
			}
		}
	default:
		return chat.Text(server.Config.Messages.UnknownCommand)
	}
}

func ParseTarget(executor *Player, arg string) (bool, *Player) {
	if len(arg) == 2 && strings.HasPrefix(arg, "@") {
		t := strings.TrimPrefix(arg, "@")
		switch t {
		case "s":
			{
				return true, executor
			}
		default:
			return false, nil
		}
	}
	if player, ok := server.Players.Players[arg]; ok {
		return true, player
	}
	if player, ok := server.Players.PlayerNames[arg]; ok {
		return true, server.Players.Players[player]
	}
	return false, nil
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
