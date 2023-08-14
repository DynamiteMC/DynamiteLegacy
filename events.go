package main

import (
	"fmt"
	"strings"

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
	header, footer := server.Playerlist.GetTexts(player)
	connection.WritePacket(pk.Marshal(0x17, pk.Identifier("minecraft:brand"), pk.String("GoCraft")))
	connection.WritePacket(pk.Marshal(packetid.ClientboundTabList, chat.Text(header), chat.Text(footer)))
	fields := []pk.FieldEncoder{
		chat.Text(server.Config.MOTD),
		pk.Boolean(false),
	}
	if server.Config.Icon.Enable {
		success, _, data := server.GetFavicon()
		if success {
			fields[1] = pk.Boolean(true)
			fields = append(fields, pk.ByteArray(data))
		}
	}
	fields = append(fields, pk.Boolean(true))
	connection.WritePacket(pk.Marshal(packetid.ClientboundServerData, fields...))
	connection.WritePacket(pk.Marshal(
		packetid.ClientboundSetChunkCacheCenter,
		pk.VarInt(0),
		pk.VarInt(0),
	))
	connection.WritePacket(pk.Marshal(packetid.ClientboundCommands, CommandGraph{}))

	max := fmt.Sprint(server.Config.MaxPlayers)
	if max == "-1" {
		max = "Unlimited"
	}
	if playerCountText != nil && playerContainer != nil {
		playerCountText.ParseMarkdown(fmt.Sprintf("### %d/%s players", len(server.Players), max))
		playerContainer.Refresh()
	}

	group, prefix, suffix := server.GetGroup(player.UUID)

	server.BroadcastMessage(chat.Text(ParsePlaceholders(server.Config.Messages.PlayerJoin, Placeholders{PlayerName: player.Name, PlayerPrefix: prefix, PlayerSuffix: suffix, PlayerGroup: group})))
	server.Playerlist.AddPlayer(player)
}

func OnPlayerLeave(params ...interface{}) {
	player := params[0].(Player)
	delete(server.Players, player.UUID)

	max := fmt.Sprint(server.Config.MaxPlayers)
	if max == "-1" {
		max = "Unlimited"
	}
	if playerCountText != nil && playerContainer != nil {
		playerCountText.ParseMarkdown(fmt.Sprintf("### %d/%s players", len(server.Players), max))
		playerContainer.Refresh()
	}

	group, prefix, suffix := server.GetGroup(player.UUID)

	server.BroadcastMessage(chat.Text(ParsePlaceholders(server.Config.Messages.PlayerLeave, Placeholders{PlayerName: player.Name, PlayerPrefix: prefix, PlayerSuffix: suffix, PlayerGroup: group})))
	server.Playerlist.RemovePlayer(player)
}

func OnPlayerChatMessage(params ...interface{}) {
	if !server.Config.Chat.Enable {
		return
	}
	player := params[0].(Player)
	if !server.HasPermissions(player.UUID, []string{"server.chat"}) {
		return
	}
	content := params[1].(pk.String)

	group, prefix, suffix := server.GetGroup(player.UUID)

	data := ParsePlaceholders(server.Config.Chat.Format, Placeholders{PlayerName: player.Name, PlayerPrefix: prefix, PlayerSuffix: suffix, Message: fmt.Sprint(content), PlayerGroup: group})
	if server.Config.Chat.Colors && server.HasPermissions(player.UUID, []string{"server.chat.colors"}) {
		data = strings.ReplaceAll(data, "&", "§")
	}
	server.BroadcastMessage(chat.Text(data))
}

func OnPlayerCommand(params ...interface{}) {
	player := params[0].(Player)
	command := params[1].(pk.String)
	server.BroadcastMessageAdmin(player.UUID, chat.Text(fmt.Sprintf("Player %s (%s) executed command %s", player.Name, player.UUID, command)))
	server.Message(player.UUID, server.Command(player.UUID, fmt.Sprint(command)))
}
