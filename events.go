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
	/*connection.WritePacket(pk.Marshal(packetid.ClientboundCommands, pk.Array([]pk.FieldEncoder{
		pk.Array([]pk.FieldEncoder{
			pk.Byte(0x0),
			pk.VarInt(1),
			pk.Array([]pk.VarInt{pk.VarInt(1)}),
		}),
		pk.Array([]pk.FieldEncoder{
			pk.Byte(0x01),
			pk.VarInt(0),
			pk.Array([]pk.VarInt{}),
			pk.VarInt(0),
			pk.String("me"),
		}),
	}), pk.VarInt(0)))*/

	max := fmt.Sprint(server.Config.MaxPlayers)
	if max == "-1" {
		max = "Unlimited"
	}
	if playerCountText != nil && playerContainer != nil {
		playerCountText.ParseMarkdown(fmt.Sprintf("### %d/%s players", len(server.Players), max))
		playerContainer.Refresh()
	}

	prefix, suffix := server.GetPrefixSuffix(player.UUID)

	server.BroadcastMessage(chat.Text(ParsePlaceholders(server.Config.Messages.PlayerJoin, Placeholders{PlayerName: player.Name, PlayerPrefix: prefix, PlayerSuffix: suffix})))
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

	prefix, suffix := server.GetPrefixSuffix(player.UUID)

	server.BroadcastMessage(chat.Text(ParsePlaceholders(server.Config.Messages.PlayerLeave, Placeholders{PlayerName: player.Name, PlayerPrefix: prefix, PlayerSuffix: suffix})))
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

	prefix, suffix := server.GetPrefixSuffix(player.UUID)

	data := ParsePlaceholders(server.Config.Chat.Format, Placeholders{PlayerName: player.Name, PlayerPrefix: prefix, PlayerSuffix: suffix, Message: fmt.Sprint(content)})
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
