package main

import (
	"bytes"
	"strings"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/yggdrasil/user"
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

const (
	CHAT_ENABLED = iota
	CHAT_COMMANDS_ONLY
	CHAT_HIDDEN
)

const (
	LEFT_HAND = iota
	RIGHT_HAND
)

type ClientData struct {
	Locale              pk.String
	ViewDistance        pk.Byte
	ChatMode            pk.VarInt
	ChatColors          pk.Boolean
	DisplayedSkinParts  pk.UnsignedByte
	MainHand            pk.VarInt
	EnableTextFiltering pk.Boolean
	AllowServerListings pk.Boolean
	Brand               pk.String
}

type Argument struct {
	Name                string
	Redirect            int
	RequiredPermissions []string
	SuggestionsType     string
	ParserID            string
}

type Command struct {
	Name                string
	Redirect            int
	RequiredPermissions []string
	Arguments           []Argument
	Executable          bool
}

type Player struct {
	Name       string `json:"name"`
	UUID       string `json:"id"`
	UUIDb      pk.UUID
	Connection net.Conn
	Properties []user.Property
	Client     ClientData
	IP         string
}

type Playerlist struct{}

type Server struct {
	Players    map[string]Player
	PlayerIDs  []string
	Events     Events
	Config     *Config
	Logger     Logger
	Playerlist Playerlist
	StartTime  int64
}

func (server Server) BroadcastMessage(message chat.Message) {
	server.Logger.Print(message.String())
	for _, player := range server.Players {
		server.Message(player.UUID, message)
	}
}

func (server Server) GetPrefixSuffix(playerId string) (string, string) {
	player := getPlayer(playerId)
	group := getGroup(player.Group)
	return group.Prefix, group.Suffix
}

func (server Server) BroadcastMessageAdmin(playerId string, message chat.Message) {
	server.Logger.Print(message.String())
	op := LoadPlayerList("ops.json")
	ops := make(map[string]Player)
	for i := 0; i < len(op); i++ {
		ops[op[i].UUID] = op[i]
	}
	for _, player := range server.Players {
		if ops[player.UUID].UUID == player.UUID && player.UUID != playerId {
			server.Message(player.UUID, message)
		} else {
			continue
		}
	}
}

func (server Server) BroadcastPacket(packet pk.Packet) {
	for _, player := range server.Players {
		player.Connection.WritePacket(packet)
	}
}

func (server Server) Message(id string, message chat.Message) {
	player := server.Players[id]
	if player.UUID != id {
		return
	}
	player.Connection.WritePacket(pk.Marshal(packetid.ClientboundSystemChat, message, pk.Boolean(false)))
}

func (playerlist Playerlist) AddPlayer(player Player) {
	addPlayerAction := NewPlayerInfoAction(
		PlayerInfoAddPlayer,
		PlayerInfoUpdateListed,
	)
	var buf bytes.Buffer
	_, _ = addPlayerAction.WriteTo(&buf)
	_, _ = pk.VarInt(len(server.Players)).WriteTo(&buf)
	for _, player := range server.Players {
		_, _ = pk.UUID(player.UUIDb).WriteTo(&buf)
		_, _ = pk.String(player.Name).WriteTo(&buf)
		_, _ = pk.Array(player.Properties).WriteTo(&buf)
		_, _ = pk.Boolean(true).WriteTo(&buf)
	}
	server.BroadcastPacket(pk.Packet{ID: int32(packetid.ClientboundPlayerInfoUpdate), Data: buf.Bytes()})
}

func (playerlist Playerlist) RemovePlayer(player Player) {
	server.BroadcastPacket(pk.Marshal(packetid.ClientboundPlayerInfoRemove, pk.Array([]pk.UUID{player.UUIDb})))
}

func (playerlist Playerlist) GetTexts(player Player) (string, string) {
	prefix, suffix := server.GetPrefixSuffix(player.UUID)
	header := ParsePlaceholders(strings.Join(server.Config.Tablist.Header, "\n"), Placeholders{PlayerName: player.Name, PlayerPrefix: prefix})
	footer := ParsePlaceholders(strings.Join(server.Config.Tablist.Footer, "\n"), Placeholders{PlayerName: player.Name, PlayerSuffix: suffix})
	return header, footer
}

type Placeholders struct {
	PlayerName   string
	Message      string
	PlayerPrefix string
	PlayerSuffix string
}

func ParsePlaceholders(str string, placeholders Placeholders) string {
	str = strings.ReplaceAll(str, "%player%", placeholders.PlayerName)
	str = strings.ReplaceAll(str, "%message%", placeholders.Message)
	str = strings.ReplaceAll(str, "%player_prefix%", placeholders.PlayerPrefix)
	str = strings.ReplaceAll(str, "%player_suffix%", placeholders.PlayerSuffix)
	return str
}
