package main

import (
	"strings"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
)

type Events struct {
	_events map[string][]func(...interface{})
}

func (emitter Events) AddListener(key string, action func(...interface{})) {
	if emitter._events[key] == nil {
		emitter._events[key] = make([]func(...interface{}), 0)
	}
	emitter._events[key] = append(emitter._events[key], action)
}

func (emitter Events) RemoveListener(key string, index int) {
	emitter._events[key][index] = nil
}

func (emitter Events) RemoveAllListeners(key string) {
	delete(emitter._events, key)
}

func (emitter Events) Emit(key string, data ...interface{}) {
	for _, action := range emitter._events[key] {
		if action == nil {
			continue
		}
		action(data...)
	}
}

type Player struct {
	Name       string `json:"name"`
	UUID       string `json:"id"`
	Connection net.Conn
}

type Server struct {
	Players map[string]Player
	Events  Events
	Config  *Config
}

func (server Server) BroadcastMessage(message chat.Message) {
	logger.Print(message.String())
	for _, player := range server.Players {
		player.Connection.WritePacket(pk.Marshal(packetid.ClientboundSystemChat, message, pk.Boolean(false)))
	}
}

func ParsePlaceholders(str string, playerName string) string {
	return strings.ReplaceAll(str, "%player%", playerName)
}
