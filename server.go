package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"image"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"net"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/nbt"
	mcnet "github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/save"
	"github.com/Tnze/go-mc/save/region"
	"github.com/Tnze/go-mc/yggdrasil/user"
)

const (
	CHAT_ENABLED = iota
	CHAT_COMMANDS_ONLY
	CHAT_HIDDEN
)

const (
	LEFT_HAND = iota
	RIGHT_HAND
)

const (
	FAVICON_NOTFOUND = iota
	FAVICON_INVALID_FORMAT
	FAVICON_INVALID_DIMENSIONS
)

const (
	PLUGINCODE_INFO = iota
	PLUGINCODE_LOG
)

type Events struct {
	_Events map[string][]func(...interface{})
}

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
	Connection mcnet.Conn
	Properties []user.Property
	Client     ClientData
	IP         string
}

type Playerlist struct{}

type Server struct {
	Players       map[string]Player
	PlayerIDs     []string
	Events        Events
	Config        *Config
	Logger        Logger
	Playerlist    Playerlist
	StartTime     int64
	Whitelist     []Player
	OPs           []Player
	BannedPlayers []Player
	BannedIPs     []string
	Favicon       []byte
	Level         save.Level
	TCPListener   *mcnet.Listener
	UDPListener   *net.UDPConn
	EntityCounter int
}

func (emitter Events) AddListener(key string, action func(...interface{})) {
	if emitter._Events[key] == nil {
		emitter._Events[key] = make([]func(...interface{}), 0)
	}
	emitter._Events[key] = append(emitter._Events[key], action)
}

func (emitter Events) RemoveListener(key string, index int) {
	emitter._Events[key][index] = nil
}

func (emitter Events) RemoveAllListeners(key string) {
	delete(emitter._Events, key)
}

func (emitter Events) Emit(key string, data ...interface{}) {
	for _, action := range emitter._Events[key] {
		if action == nil {
			continue
		}
		action(data...)
	}
}

func (server *Server) ParseWorldData() {
	if _, e := os.Stat("world"); os.IsNotExist(e) {
		server.Logger.Error("Please import a world folder from a singleplayer world or a vanilla server")
		os.Exit(1)
	}
	b, _ := os.Open("world/level.dat")
	data, _ := gzip.NewReader(b)
	decoder := nbt.NewDecoder(data)
	decoder.DisallowUnknownFields()
	_, err := decoder.Decode(&server.Level)
	if err != nil {
		server.Logger.Error("Failed to parse world data")
		os.Exit(1)
	}
	server.Logger.Debug("Parsed world data")
}

func (server *Server) NewEntityID() int {
	server.EntityCounter += 1
	return server.EntityCounter
}

func (server Server) GetChunk(pos [2]int32) *level.Chunk {
	rx, rz := region.At(int(pos[0]), int(pos[1]))
	filename := fmt.Sprintf("world/region/r.%d.%d.mca", rx, rz)
	r, err := region.Open(filename)
	if errors.Is(err, fs.ErrNotExist) {
		r, _ = region.Create(filename)
	}
	x, z := region.In(int(pos[0]), int(pos[1]))
	if !r.ExistSector(x, z) {
		return nil
	}
	data, err := r.ReadSector(x, z)
	if err != nil {
		return nil
	}
	var c save.Chunk
	err = c.Load(data)
	if err != nil {
		return nil
	}
	r.Close()
	chunk, err := level.ChunkFromSave(&c)
	if err != nil {
		return nil
	}
	return chunk
}

func (server *Server) Init() {
	server.Whitelist = LoadPlayerList("whitelist.json")
	server.OPs = LoadPlayerList("ops.json")
	server.BannedPlayers = LoadPlayerList("banned_players.json")
	server.BannedIPs = LoadIPBans()
	server.LoadAllPlugins()
	os.MkdirAll("permissions/groups", 0755)
	os.MkdirAll("permissions/players", 0755)
	os.WriteFile("permissions/groups/default.json", []byte(`{"display_name":"default","permissions":{"server.chat":true}}`), 0755)
	server.Logger.Debug("Loaded player info")
	if !server.Config.Online && !HasArg("-no_offline_warn") {
		server.Logger.Warn("Offline mode is insecure. You can disable this message using -no_offline_warn")
	}
	server.ParseWorldData()
	if server.Config.TCP.Enable {
		TCPListen()
	}
	if server.Config.UDP.Enable {
		UDPListen()
	}
	CreateEvents()
}

func (server *Server) GetFavicon() (bool, int, []byte) {
	var data []byte
	if len(server.Favicon) == 0 {
		var err error
		data, err = os.ReadFile(server.Config.Icon.Path)
		if err != nil {
			return false, 0, data
		} else {
			image, format, _ := image.DecodeConfig(bytes.NewReader(data))
			if format != "png" {
				return false, 1, data
			}
			if image.Width != 64 || image.Height != 64 {
				return false, 2, data
			}
		}
		server.Favicon = data
	} else {
		data = server.Favicon
	}
	return true, -1, data
}

func (server Server) BroadcastMessage(message chat.Message) {
	server.Logger.Print(message.String())
	for _, player := range server.Players {
		server.Message(player.UUID, message)
	}
}

func (server Server) GetGroup(playerId string) (string, string, string) {
	player := getPlayer(playerId)
	group := getGroup(player.Group)
	return group.DisplayName, group.Prefix, group.Suffix
}

func (server Server) LoadAllPlugins() {
	os.Mkdir("plugins", 0755)
	plugins, _ := os.ReadDir("plugins")
	for _, plugin := range plugins {
		server.LoadPlugin(plugin.Name())
	}
}

func (server Server) LoadPlugin(fileName string) {
	server.Logger.Info("Loading plugin", fileName)
	path, err := exec.LookPath(fmt.Sprintf("./plugins/%s", fileName))
	if err != nil {
		server.Logger.Error("Could not load plugin", fileName)
	}
	cmd := exec.Command(path)
	stdout, _ := cmd.StdoutPipe()
	cmd.Start()
	scanner := bufio.NewScanner(stdout)
	go func() {
		for scanner.Scan() {
			command := scanner.Text()
			if !strings.HasPrefix(command, "GoCraft|Message") {
				cmd.Process.Kill()
				server.Logger.Error("Failed to load plugin", fileName, "Reason: invalid message")
				return
			}
			command = strings.TrimSpace(strings.TrimPrefix(command, "GoCraft|Message"))
			c := strings.Split(command, "")[0]
			code, err := strconv.Atoi(c)
			if err != nil {
				cmd.Process.Kill()
				server.Logger.Error("Failed to load plugin", fileName, "Reason: invalid message")
				return
			}
			command = strings.TrimSpace(strings.TrimPrefix(command, c))
			switch code {
			case PLUGINCODE_LOG:
				{
					var message chat.Message
					err := message.UnmarshalJSON([]byte(command))
					if err != nil {
						return
					}
					server.Logger.Print(message)
				}
			}
		}
	}()
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
	group, prefix, suffix := server.GetGroup(player.UUID)
	header := ParsePlaceholders(strings.Join(server.Config.Tablist.Header, "\n"), Placeholders{PlayerName: player.Name, PlayerPrefix: prefix, PlayerGroup: group})
	footer := ParsePlaceholders(strings.Join(server.Config.Tablist.Footer, "\n"), Placeholders{PlayerName: player.Name, PlayerSuffix: suffix, PlayerGroup: group})
	return header, footer
}

type Placeholders struct {
	PlayerName   string
	Message      string
	PlayerGroup  string
	PlayerPrefix string
	PlayerSuffix string
}

func ParsePlaceholders(str string, placeholders Placeholders) string {
	str = strings.ReplaceAll(str, "%player%", placeholders.PlayerName)
	str = strings.ReplaceAll(str, "%message%", placeholders.Message)
	str = strings.ReplaceAll(str, "%player_prefix%", placeholders.PlayerPrefix)
	str = strings.ReplaceAll(str, "%player_suffix%", placeholders.PlayerSuffix)
	str = strings.ReplaceAll(str, "%player_group%", placeholders.PlayerGroup)
	return str
}
