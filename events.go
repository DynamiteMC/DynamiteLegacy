package main

import (
	"fmt"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
)

func CreateEvents() {
	server.Events.AddListener("PlayerJoin", OnPlayerJoin)
	server.Events.AddListener("PlayerLeave", OnPlayerLeave)
	server.Events.AddListener("PlayerChatMessage", OnPlayerChatMessage)
	server.Events.AddListener("PlayerCommand", OnPlayerCommand)
}

func OnPlayerJoin(params ...interface{}) {
	player := params[0].(Player)
	connection := params[1].(net.Conn)
	header, footer := server.Playerlist.GetTexts(player.Name)
	connection.WritePacket(pk.Marshal(0x17, pk.Identifier("minecraft:brand"), pk.String("GoCraft")))
	connection.WritePacket(pk.Marshal(packetid.ClientboundTabList, chat.Text(header), chat.Text(footer)))
	connection.WritePacket(pk.Marshal(packetid.ClientboundCommands, pk.Array([]pk.FieldEncoder{}), pk.VarInt(0)))
	server.BroadcastMessage(chat.Text(ParsePlayerName(server.Config.Messages.PlayerJoin, player.Name)))
	server.Playerlist.AddPlayer(player)
}

func OnPlayerLeave(params ...interface{}) {
	player := params[0].(Player)
	server.BroadcastMessage(chat.Text(ParsePlayerName(server.Config.Messages.PlayerLeave, player.Name)))
	server.Playerlist.RemovePlayer(player)
	delete(server.Players, player.UUID)
}

func OnPlayerChatMessage(params ...interface{}) {
	if !server.Config.Chat.Enable {
		return
	}
	player := params[0].(Player)
	content := params[1].(pk.String)
	data := ParseChatMessage(server.Config.Chat.Format, player.Name, fmt.Sprint(content), server.Config.Chat.Colors)
	server.BroadcastMessage(chat.Text(data))
}

func OnPlayerCommand(params ...interface{}) {
	player := params[0].(Player)
	command := params[1].(pk.String)
	server.BroadcastMessageAdmin(player.UUID, chat.Text(fmt.Sprintf("Player %s (%s) executed command %s", player.Name, player.UUID, command)))
	server.Message(player.UUID, Command(fmt.Sprint(command)))
}
