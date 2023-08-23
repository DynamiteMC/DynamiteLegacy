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
	player := params[0].(*Player)
	connection := params[1].(net.Conn)
	header, footer := server.Playerlist.GetTexts(player)
	connection.WritePacket(pk.Marshal(0x17, pk.Identifier("minecraft:brand"), pk.String("Dynamite")))
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
	connection.WritePacket(pk.Marshal(packetid.ClientboundCommands, CommandGraph{player.UUID.String}))

	max := fmt.Sprint(server.Config.MaxPlayers)
	if max == "-1" {
		max = "Unlimited"
	}
	if playerCountText != nil && playerContainer != nil {
		playerCountText.ParseMarkdown(fmt.Sprintf("### %d/%s players", len(server.Players.Players), max))
		playerContainer.Refresh()
	}

	group, prefix, suffix := server.GetGroup(player.UUID.String)

	server.BroadcastMessage(chat.Text(ParsePlaceholders(server.Config.Messages.PlayerJoin, Placeholders{PlayerName: player.Name, PlayerPrefix: prefix, PlayerSuffix: suffix, PlayerGroup: group})))
	server.Playerlist.AddPlayer(player)
	server.BroadcastPacketExcept(pk.Marshal(packetid.ClientboundAddPlayer,
		pk.VarInt(player.EntityID),
		player.UUID.Binary,
		pk.Double(player.Position[0]),
		pk.Double(player.Position[1]),
		pk.Double(player.Position[2]),
		pk.Angle(player.Rotation[0]),
		pk.Angle(player.Rotation[1]),
	), player.UUID.String)
	server.Players.Lock()
	for _, p := range server.Players.Players {
		if p.UUID == player.UUID {
			continue
		}
		connection.WritePacket(pk.Marshal(packetid.ClientboundAddPlayer,
			pk.VarInt(p.EntityID),
			p.UUID.Binary,
			pk.Double(p.Position[0]),
			pk.Double(p.Position[1]),
			pk.Double(p.Position[2]),
			pk.Angle(p.Rotation[0]),
			pk.Angle(p.Rotation[1]),
		))
	}
	server.Players.Unlock()
}

func OnPlayerLeave(params ...interface{}) {
	player := params[0].(*Player)
	delete(server.Players.Players, player.UUID.String)
	delete(server.Players.PlayerNames, player.Name)
	max := fmt.Sprint(server.Config.MaxPlayers)
	group, prefix, suffix := server.GetGroup(player.UUID.String)
	message := server.Config.Messages.PlayerLeave
	if max == "-1" {
		max = "Unlimited"
	}
	if playerCountText != nil && playerContainer != nil {
		playerCountText.ParseMarkdown(fmt.Sprintf("### %d/%s players", len(server.Players.Players), max))
		playerContainer.Refresh()
	}
	server.Playerlist.RemovePlayer(player)
	server.BroadcastMessage(chat.Text(ParsePlaceholders(message, Placeholders{PlayerName: player.Name, PlayerPrefix: prefix, PlayerSuffix: suffix, PlayerGroup: group})))
}

func OnPlayerChatMessage(params ...interface{}) {
	if !server.Config.Chat.Enable {
		return
	}
	player := params[0].(*Player)
	if !server.HasPermissions(player.UUID.String, []string{"server.chat"}) {
		return
	}
	packet := params[1].(pk.Packet)
	server.BroadcastPlayerMessage(player, packet)
}

func OnPlayerCommand(params ...interface{}) {
	player := params[0].(*Player)
	command := params[1].(pk.String)
	server.BroadcastMessageAdmin(player.UUID.String, chat.Text(fmt.Sprintf("Player %s (%s) executed command %s", player.Name, player.UUID.String, command)))
	server.Message(player.UUID.String, server.Command(player.UUID.String, fmt.Sprint(command)))
}
