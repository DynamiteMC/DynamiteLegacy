package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"gocraft/logger"
	"image"
	"io"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/nbt"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/save"
	"github.com/Tnze/go-mc/save/region"
)

func (lc *LoadedChunk) AddViewer(player string) {
	lc.Lock()
	defer lc.Unlock()
	for _, v2 := range lc.Viewers {
		if v2 == player {
			return
		}
	}
	lc.Viewers = append(lc.Viewers, player)
}

func (lc *LoadedChunk) RemoveViewer(player string) bool {
	lc.Lock()
	defer lc.Unlock()
	for i, v2 := range lc.Viewers {
		if v2 == player {
			last := len(lc.Viewers) - 1
			lc.Viewers[i] = lc.Viewers[last]
			lc.Viewers = lc.Viewers[:last]
			return true
		}
	}
	return false
}

var loadList [][2]int32

var radiusIdx []int

func (player *Player) CalculateLoadingQueue() {
	player.LoadQueue = player.LoadQueue[:0]
	rd := player.Client.ViewDistance
	if rd > pk.Byte(server.Config.ViewDistance) {
		rd = pk.Byte(server.Config.ViewDistance)
	}
	for _, v := range loadList[:radiusIdx[rd]] {
		pos := [2]int32{player.ChunkPos[0], player.ChunkPos[2]}
		pos[0], pos[1] = pos[0]+v[0], pos[1]+v[1]
		if _, ok := player.LoadedChunks[pos]; !ok {
			player.LoadQueue = append(player.LoadQueue, pos)
		}
	}
}

func (p *Player) CalculateUnusedChunks() {
	p.UnloadQueue = p.UnloadQueue[:0]
	for chunk := range p.LoadedChunks {
		player := [2]int32{p.ChunkPos[0], p.ChunkPos[2]}
		r := p.Client.ViewDistance
		if distance2i([2]int32{chunk[0] - player[0], chunk[1] - player[1]}) > float64(r) {
			p.UnloadQueue = append(p.UnloadQueue, chunk)
		}
	}
}

func InitLoader() {
	var maxR = int32(server.Config.ViewDistance)

	for x := -maxR; x <= maxR; x++ {
		for z := -maxR; z <= maxR; z++ {
			pos := [2]int32{x, z}
			if distance2i(pos) < float64(maxR) {
				loadList = append(loadList, pos)
			}
		}
	}
	sort.Slice(loadList, func(i, j int) bool {
		return distance2i(loadList[i]) < distance2i(loadList[j])
	})

	radiusIdx = make([]int, maxR+1)
	for i, v := range loadList {
		r := int32(math.Ceil(distance2i(v)))
		if r > maxR {
			break
		}
		radiusIdx[r] = i
	}
}

func distance2i(pos [2]int32) float64 {
	return math.Sqrt(float64(pos[0]*pos[0]) + float64(pos[1]*pos[1]))
}
func (w *World) LoadChunk(pos [2]int32) bool {
	c := server.GetChunk(pos)
	if c == nil {
		c = level.EmptyChunk(24)
		c.Status = level.StatusFull
	}
	w.Chunks[pos] = &LoadedChunk{Chunk: c}
	return true
}

func (w *World) UnloadChunk(pos [2]int32) {
	for _, player := range server.Players {
		if player.Data.Dimension != w.Name {
			continue
		}
		player.Connection.WritePacket(pk.Marshal(packetid.ClientboundForgetLevelChunk, level.ChunkPos(pos)))
	}
	delete(w.Chunks, pos)
}

func (server Server) PlayersAsBase() []PlayerBase {
	players := make([]PlayerBase, 0)
	for _, player := range server.Players {
		players = append(players, PlayerBase{
			UUID: player.UUID.String,
			Name: player.Name,
		})
	}
	return players
}

func (server Server) GetPlayerData(playerId string) *PlayerData {
	var player PlayerData
	path := fmt.Sprintf("world/playerdata/%s.dat", playerId)
	file, err := os.Open(path)
	data, _ := gzip.NewReader(file)
	if err != nil && errors.Is(err, fs.ErrNotExist) {
		return nil
	} else {
		defer file.Close()
		decoder := nbt.NewDecoder(data)
		decoder.Decode(&player)
	}
	return &player
}

func (server Server) WritePlayerData(playerId string, data PlayerData) error {
	path := fmt.Sprintf("world/playerdata/%s.dat", playerId)
	var w bytes.Buffer
	writer := gzip.NewWriter(&w)
	f, _ := nbt.Marshal(data)
	writer.Write(f)
	writer.Close()
	os.WriteFile(path, w.Bytes(), 0755)
	return nil
}

func (graph CommandGraph) WriteTo(w io.Writer) (int64, error) {
	entries := []pk.Tuple{{}}
	var rootChildren []int32
	var nodes []Node
	i := 1
	for _, command := range server.Commands {
		if !server.HasPermissions(graph.PlayerID, command.RequiredPermissions) {
			continue
		}
		nodes = append(nodes, Node{Parent: 0, Data: command})
		for _, argument := range command.Arguments {
			nodes = append(nodes, Node{Parent: i, Data: argument})
			i++
		}
		i++
	}
	for index, node := range nodes {
		command, isCommand := node.Data.(Command)
		argument, isArgument := node.Data.(Argument)
		nodes[index].EntryIndex = len(entries)
		if isCommand {
			flags := 0x01
			if len(command.Arguments) == 0 {
				flags |= 0x04
			}
			entries = append(entries, pk.Tuple{
				pk.Byte(flags),
				pk.Array((*[]pk.VarInt)(unsafe.Pointer(&[]int32{}))),
				pk.String(command.Name),
			})
			rootChildren = append(rootChildren, int32(index+1))
		} else if isArgument {
			flags := 0x02
			if argument.SuggestionsType != "" {
				flags |= 0x10
			}
			parent := nodes[node.Parent-1]
			command, isCommand = parent.Data.(Command)
			arg, isArg := parent.Data.(Argument)
			if !isCommand && !isArg {
				continue
			}
			//var isNextArg bool
			/*if len(nodes) <= i+1 {
				isNextArg = false
			} else {
				_, isNextArg = nodes[i+1].Data.(Argument)
			}*/
			/*if _, ok := parent.Data.(Command); ok && isNextArg {
				nodes[i+1].Parent = node.Parent
			}*/
			parent.Children = append(parent.Children, len(entries))
			if isCommand {
				entries[parent.EntryIndex] = pk.Tuple{
					pk.Byte(0x01),
					pk.Array((*[]pk.VarInt)(unsafe.Pointer(&parent.Children))),
					pk.String(command.Name),
				}
			} else if isArg {
				fl := 0x02
				if arg.SuggestionsType != "" {
					fl |= 0x10
				}
				entries[parent.EntryIndex] = pk.Tuple{
					pk.Byte(fl),
					pk.Array((*[]pk.VarInt)(unsafe.Pointer(&parent.Children))),
					pk.String(arg.Name),
					pk.VarInt(arg.Parser.ID),
					pk.Opt{
						Has:   func() bool { return arg.Parser.Properties != nil },
						Field: arg.Parser.Properties,
					},
					pk.Opt{
						Has:   func() bool { return arg.SuggestionsType != "" },
						Field: pk.String(arg.SuggestionsType),
					},
				}
			}
			entries = append(entries, pk.Tuple{
				pk.Byte(flags),
				pk.Array((*[]pk.VarInt)(unsafe.Pointer(&[]int32{}))),
				pk.String(argument.Name),
				pk.VarInt(argument.Parser.ID),
				pk.Opt{
					Has:   func() bool { return argument.Parser.Properties != nil },
					Field: argument.Parser.Properties,
				},
				pk.Opt{
					Has:   func() bool { return argument.SuggestionsType != "" },
					Field: pk.String(argument.SuggestionsType),
				},
			})
		}
	}
	entries[0] = pk.Tuple{
		pk.Byte(0x0),
		pk.Array((*[]pk.VarInt)(unsafe.Pointer(&rootChildren))),
	}
	return pk.Tuple{
		pk.Array(entries),
		pk.VarInt(0),
	}.WriteTo(w)
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
func (server Server) IsOP(id string) (bool, PlayerBase) {
	for _, op := range server.OPs {
		if op.UUID == id || op.Name == id {
			return true, op
		}
	}
	return false, PlayerBase{}
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
	server.Worlds["minecraft:overworld"] = World{Name: "minecraft:overworld", Chunks: make(map[[2]int32]*LoadedChunk)}
	InitLoader()
	go server.Worlds["minecraft:overworld"].TickLoop()
	server.Logger.Debug("Parsed world data")
}

func (server *Server) NewEntityID() int {
	server.EntityCounter += 1
	return server.EntityCounter
}

func (server *Server) NewTeleportID() int {
	server.TeleportCounter += 1
	return server.TeleportCounter
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
	server.Worlds = make(map[string]World)
	server.LoadAllPlugins()
	os.MkdirAll("permissions/groups", 0755)
	os.MkdirAll("permissions/players", 0755)
	os.WriteFile("permissions/groups/default.json", []byte(`{"display_name":"default","permissions":{"server.chat":true}}`), 0755)
	server.Logger.Debug("Loaded player info")
	if !server.Config.Online && !logger.HasArg("-no_offline_warn") {
		server.Logger.Warn("Offline mode is insecure. You can disable this message using -no_offline_warn")
	}
	server.ParseWorldData()
	TCPListen()
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
		server.Message(player.UUID.String, message)
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
	server.Logger.Info("Loading plugin %s", fileName)
	path, err := exec.LookPath(fmt.Sprintf("./plugins/%s", fileName))
	if err != nil {
		server.Logger.Error("Could not load plugin %s", fileName)
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
				server.Logger.Error("Failed to load plugin %s Reason: invalid message", fileName)
				return
			}
			command = strings.TrimSpace(strings.TrimPrefix(command, "GoCraft|Message"))
			c := strings.Split(command, "")[0]
			code, err := strconv.Atoi(c)
			if err != nil {
				cmd.Process.Kill()
				server.Logger.Error("Failed to load plugin %s Reason: invalid message", fileName)
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
					server.Logger.Print("%v", message)
				}
			}
		}
	}()
}

func (server Server) BroadcastMessageAdmin(playerId string, message chat.Message) {
	server.Logger.Print(message.String())
	op := LoadPlayerList("ops.json")
	ops := make(map[string]PlayerBase)
	for i := 0; i < len(op); i++ {
		ops[op[i].UUID] = PlayerBase{
			Name: op[i].Name,
			UUID: op[i].UUID,
		}
	}
	for _, player := range server.Players {
		if ops[player.UUID.String].UUID == player.UUID.String && player.UUID.String != playerId {
			server.Message(player.UUID.String, message)
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
	if player.UUID.String != id {
		return
	}
	player.Connection.WritePacket(pk.Marshal(packetid.ClientboundSystemChat, message, pk.Boolean(false)))
}

func (playerlist Playerlist) AddPlayer(player *Player) {
	addPlayerAction := NewPlayerInfoAction(
		PlayerInfoAddPlayer,
		PlayerInfoUpdateListed,
	)
	var buf bytes.Buffer
	_, _ = addPlayerAction.WriteTo(&buf)
	_, _ = pk.VarInt(len(server.Players)).WriteTo(&buf)
	for _, player := range server.Players {
		_, _ = pk.UUID(player.UUID.Binary).WriteTo(&buf)
		_, _ = pk.String(player.Name).WriteTo(&buf)
		_, _ = pk.Array(player.Properties).WriteTo(&buf)
		_, _ = pk.Boolean(true).WriteTo(&buf)
	}
	server.BroadcastPacket(pk.Packet{ID: int32(packetid.ClientboundPlayerInfoUpdate), Data: buf.Bytes()})
}

func (playerlist Playerlist) RemovePlayer(player *Player) {
	server.BroadcastPacket(pk.Marshal(packetid.ClientboundPlayerInfoRemove, pk.Array([]pk.UUID{player.UUID.Binary})))
}

func (playerlist Playerlist) GetTexts(player *Player) (string, string) {
	group, prefix, suffix := server.GetGroup(player.UUID.String)
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
	str = strings.TrimSpace(str)
	return str
}
