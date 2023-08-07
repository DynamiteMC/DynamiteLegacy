package main

import (
	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
)

func CreateEvents() {
	server.Events.AddListener("PlayerJoin", OnPlayerJoin)
	server.Events.AddListener("PlayerLeave", OnPlayerLeave)
}

func OnPlayerJoin(params ...interface{}) {
	player := params[0].(Player)
	connection := params[1].(net.Conn)
	header, footer := server.Playerlist.GetTexts(player.Name)
	connection.WritePacket(pk.Marshal(packetid.ClientboundTabList, chat.Text(header), chat.Text(footer)))
	server.BroadcastMessage(chat.Text(ParsePlaceholders(server.Config.Messages.PlayerJoin, player.Name)))
	server.Playerlist.AddPlayer(player)
}

func OnPlayerLeave(params ...interface{}) {
	player := params[0].(Player)
	server.BroadcastMessage(chat.Text(ParsePlaceholders(server.Config.Messages.PlayerLeave, player.Name)))
	server.Playerlist.RemovePlayer(player)
	delete(server.Players, player.UUID)
}
