package main

import (
	"github.com/Tnze/go-mc/chat"
)

func CreateEvents() {
	server.Events.AddListener("PlayerJoin", OnPlayerJoin)
	server.Events.AddListener("PlayerLeave", OnPlayerLeave)
}

func OnPlayerJoin(params ...interface{}) {
	player := params[0].(Player)
	server.BroadcastMessage(chat.Message{Text: ParsePlaceholders(server.Config.Messages.PlayerJoin, player.Name)})
	server.Playerlist.AddPlayer(player)
}

func OnPlayerLeave(params ...interface{}) {
	player := params[0].(Player)
	server.BroadcastMessage(chat.Message{Text: ParsePlaceholders(server.Config.Messages.PlayerLeave, player.Name)})
	delete(server.Players, player.UUID)
}
