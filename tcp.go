package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"

	r "math/rand"
	"time"

	"encoding/binary"
	"fmt"
	"image"

	_ "image/png"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Tnze/go-mc/chat"
	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/nbt"
	"github.com/Tnze/go-mc/net"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/offline"
	"github.com/Tnze/go-mc/server/auth"
	"github.com/Tnze/go-mc/yggdrasil/user"
	"github.com/go-mc/server/world"
)

type HeightMap struct {
	MotionBlocking []int64 `nbt:"MOTION_BLOCKING"`
}

type MojangLoginHandler struct {
	privateKey     atomic.Pointer[rsa.PrivateKey]
	lockPrivateKey sync.Mutex
}

const (
	PROTOCOL_1_20   = 763
	PROTOCOL_1_19_4 = 762
	PROTOCOL_1_19_3 = 761
	PROTOCOL_1_19_1 = 760
	PROTOCOL_1_19   = 759
	PROTOCOL_1_18_2 = 758
	PROTOCOL_1_18   = 757
	PROTOCOL_1_17_1 = 756
	PROTOCOL_1_17   = 755
	PROTOCOL_1_16_4 = 754
	PROTOCOL_1_16_3 = 753
	PROTOCOL_1_16_1 = 736
	PROTOCOL_1_16   = 735
	PROTOCOL_1_15_2 = 578
	PROTOCOL_1_15_1 = 575
	PROTOCOL_1_15   = 573
	PROTOCOL_1_14_4 = 498
	PROTOCOL_1_14_3 = 490
	PROTOCOL_1_14_2 = 485
	PROTOCOL_1_14   = 477
	PROTOCOL_1_13_2 = 404
	PROTOCOL_1_13_1 = 401
	PROTOCOL_1_13   = 393
	PROTOCOL_1_12_2 = 340
	PROTOCOL_1_12_1 = 338
	PROTOCOL_1_12   = 335
	PROTOCOL_1_11_1 = 316
	PROTOCOL_1_11   = 315
	PROTOCOL_1_10   = 210
	PROTOCOL_1_9_3  = 110
	PROTOCOL_1_9_2  = 109
	PROTOCOL_1_9_1  = 108
	PROTOCOL_1_9    = 107
	PROTOCOL_1_8    = 47
	PROTOCOL_1_7_6  = 5
	PROTOCOL_1_7_2  = 4
	PROTOCOL_1_7    = 3
)

func (d *MojangLoginHandler) getPrivateKey() (key *rsa.PrivateKey, err error) {
	key = d.privateKey.Load()
	if key != nil {
		return
	}

	d.lockPrivateKey.Lock()
	defer d.lockPrivateKey.Unlock()

	key = d.privateKey.Load()
	if key == nil {
		key, err = rsa.GenerateKey(rand.Reader, 1024)
		if err != nil {
			return
		}
		d.privateKey.Store(key)
	}
	return
}

func HandleTCPRequest(conn net.Conn) {
	defer conn.Close()
	var packet pk.Packet
	conn.ReadPacket(&packet)
	ip := conn.Socket.RemoteAddr().String()
	if packet.ID == 0x00 { // 1.7+
		server.Logger.Debug("[TCP] (["+ip+"]", "-> Server) Sent handshake")
		var (
			Protocol, Intention pk.VarInt
			ServerAddress       pk.String
			ServerPort          pk.UnsignedShort
		)
		err := packet.Scan(&Protocol, &ServerAddress, &ServerPort, &Intention)
		if err != nil {
			return
		}

		switch Intention {
		case 1: // Ping
			{
				handleTCPPing(conn, Protocol, ip)
			}
		case 2:
			{ // login
				if server.Config.TCP.MinProtocol != 0 && server.Config.TCP.MinProtocol > int(Protocol) {
					conn.WritePacket(pk.Marshal(packetid.LoginDisconnect, chat.Text(server.Config.Messages.ProtocolOld)))
					return
				}
				if server.Config.TCP.MaxProtocol != 0 && server.Config.TCP.MaxProtocol < int(Protocol) {
					conn.WritePacket(pk.Marshal(packetid.LoginDisconnect, chat.Text(server.Config.Messages.ProtocolNew)))
					return
				}
				var p pk.Packet
				conn.ReadPacket(&p)
				var (
					name pk.String
					uuid pk.UUID
				)
				p.Scan(&name, &uuid)
				server.Logger.Debug("[TCP] (["+ip+"]", "-> Server) Sent LoginStart packet. Username:", name)
				var id pk.UUID
				var idString string
				properties := []user.Property{}
				if server.Config.Online {
					d := MojangLoginHandler{}
					var serverKey *rsa.PrivateKey
					serverKey, err = d.getPrivateKey()
					if err != nil {
						return
					}
					var resp *auth.Resp
					resp, err = auth.Encrypt(&conn, fmt.Sprint(name), serverKey)
					if err != nil {
						return
					}
					name = pk.String(resp.Name)
					idString = fmt.Sprint(resp.ID)
					id = pk.UUID(resp.ID)
					properties = resp.Properties
				} else {
					id = pk.UUID(offline.NameToUUID(string(name)))
					idString = fmt.Sprint(offline.NameToUUID(string(name)))
				}
				server.Logger.Info("["+ip+"]", "Player", name, "("+idString+")", "is attempting to join.")
				valid := ValidatePlayer(fmt.Sprint(name), idString, strings.Split(ip, ":")[0])
				if valid != 0 {
					var reason string
					var reasonNice string
					switch valid {
					case 1:
						{
							reason = "player not in whitelist"
							reasonNice = server.Config.Messages.NotInWhitelist
						}
					case 2:
						{
							reason = "player is banned"
							reasonNice = server.Config.Messages.Banned
						}
					case 3:
						{
							reason = "server is full"
							reasonNice = server.Config.Messages.ServerFull
						}
					case 4:
						{
							reason = "already playing"
							reasonNice = server.Config.Messages.AlreadyPlaying
						}
					}
					r := chat.Text(reasonNice)
					server.Logger.Info("["+ip+"]", "Player", name, "("+idString+")", "attempt failed. reason:", reason)
					conn.WritePacket(pk.Marshal(packetid.LoginDisconnect, r))
				}
				loginSuccessFields := []pk.FieldEncoder{
					id,
					pk.String(name),
				}
				if Protocol >= PROTOCOL_1_19 {
					loginSuccessFields = append(loginSuccessFields, pk.Array(properties))
				}
				conn.WritePacket(pk.Marshal(
					packetid.LoginSuccess,
					loginSuccessFields...,
				))
				gamemode := 0
				if server.Config.Gamemode == "creative" {
					gamemode = 1
				}
				if server.Config.Gamemode == "adventure" {
					gamemode = 2
				}
				if server.Config.Gamemode == "spectator" {
					gamemode = 3
				}
				hashedSeed := [8]byte{}
				fields := []pk.FieldEncoder{pk.Int(0),
					pk.Boolean(server.Config.Hardcore),
					pk.UnsignedByte(gamemode),
					pk.Byte(-1),
					pk.Array([]pk.Identifier{
						pk.Identifier("world"),
					}),
					pk.NBT(world.NetworkCodec),
					pk.Identifier("minecraft:overworld"),
					pk.Identifier("world"),
					pk.Long(binary.BigEndian.Uint64(hashedSeed[:8])),
					pk.VarInt(server.Config.MaxPlayers), // Max players (ignored by client)
					pk.VarInt(12),                       // View Distance
					pk.VarInt(12),                       // Simulation Distance
					pk.Boolean(false),                   // Reduced Debug Info
					pk.Boolean(false),                   // Enable respawn screen
					pk.Boolean(false),                   // Is Debug
					pk.Boolean(false),                   // Is Flat
					pk.Boolean(false),                   // Has Last Death Location
				}
				if Protocol >= PROTOCOL_1_20 {
					fields = append(fields, pk.VarInt(3))
				}
				if Protocol >= PROTOCOL_1_19 {
					conn.WritePacket(pk.Marshal(
						packetid.ClientboundLogin,
						fields...,
					))
				}
				conn.WritePacket(pk.Marshal(packetid.ClientboundSetDefaultSpawnPosition,
					pk.Position{X: 100, Y: 100, Z: 100},
					pk.Float(50)))
				if Protocol >= PROTOCOL_1_20 {
					conn.WritePacket(pk.Marshal(0x3C,
						pk.Double(100),
						pk.Double(100),
						pk.Double(100),
						pk.Float(12),
						pk.Float(12),
						pk.Byte(0),
						pk.VarInt(0),
					))
				}
				data, _ := os.ReadFile("heightmap.nbt")
				var heightMap HeightMap
				nbt.Unmarshal(data, &heightMap)
				/*conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(0),
					pk.Int(0),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(0),
					pk.Int(1),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(0),
					pk.Int(2),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(0),
					pk.Int(3),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(1),
					pk.Int(0),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(1),
					pk.Int(1),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(1),
					pk.Int(2),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(1),
					pk.Int(3),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(2),
					pk.Int(0),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(2),
					pk.Int(1),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(2),
					pk.Int(2),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(2),
					pk.Int(3),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(3),
					pk.Int(0),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(3),
					pk.Int(1),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(3),
					pk.Int(2),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(3),
					pk.Int(3),
					pk.NBT(heightMap),
					pk.ByteArray{},
					pk.VarInt(0),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.BitSet([]int64{}),
					pk.VarInt(0),
					pk.VarInt(0),
				))*/
				var lastKeepAliveId int
				//var lastPacket pk.Packet
				player := Player{
					Name:       fmt.Sprint(name),
					UUID:       idString,
					UUIDb:      id,
					Connection: conn,
					Properties: properties,
					IP:         ip,
				}
				var (
					joined = false
					left   = false
				)
				for {
					var packet pk.Packet
					conn.ReadPacket(&packet)
					//lastPacket = packet
					switch packet.ID {
					case 8:
						{
							if joined {
								continue
							}
							server.Logger.Info("["+ip+"]", "Player", name, "("+idString+")", "joined the server")
							server.Players[idString] = player
							server.PlayerIDs = append(server.PlayerIDs, idString)
							packet.Scan(&player.Client.Locale,
								&player.Client.ViewDistance,
								&player.Client.ChatMode,
								&player.Client.ChatColors,
								&player.Client.DisplayedSkinParts,
								&player.Client.MainHand,
								&player.Client.EnableTextFiltering,
								&player.Client.AllowServerListings,
							)
							server.Players[idString] = player
							server.Events.Emit("PlayerJoin", player, conn)
							ticker := time.NewTicker(10 * time.Second)
							defer ticker.Stop()
							joined = true
							go func() {
								for range ticker.C {
									lastKeepAliveId = r.Intn(1000)
									conn.WritePacket(pk.Marshal(packetid.ClientboundKeepAlive, pk.Long(lastKeepAliveId)))
									server.Logger.Debug("[TCP] (Server -> ["+ip+"])", "Sent KeepAlive packet")
								}
							}()
						}
					case int32(packetid.ServerboundKeepAlive):
						{
							var id pk.Long
							packet.Scan(&id)
							if id != pk.Long(lastKeepAliveId) {
								conn.Close()
								server.Events.Emit("PlayerLeave", player)
								break
							}
							server.Logger.Debug("[TCP] (["+ip+"]", "-> Server) Sent KeepAlive packet")
						}
					case int32(packetid.ServerboundChatCommand):
						{
							var (
								command   pk.String
								timestamp pk.Long
								arguments pk.ByteArray
							)
							packet.Scan(&command, &timestamp, &arguments)
							server.Events.Emit("PlayerCommand", player, command, timestamp, arguments)
						}
					case int32(packetid.ServerboundChat):
						{
							var content pk.String
							packet.Scan(&content)
							server.Events.Emit("PlayerChatMessage", player, content)
						}
					case 0x0D:
						{
							var (
								channel pk.Identifier
								data    pk.String
							)
							packet.Scan(&channel, &data)
							if channel == "minecraft:brand" {
								player.Client.Brand = data
							}
							server.Players[idString] = player
						}
					case packetid.LoginDisconnect:
						{
							server.Logger.Info("["+ip+"]", "Player", name, "("+idString+")", "disconnected")
							conn.Close()
							if joined && !left {
								server.Events.Emit("PlayerLeave", player)
							}
							left = true
							return
						}
					}
				}
			}
		}
	} else if packet.ID == 122 { //1.6-
	}
}

func handleTCPPing(conn net.Conn, Protocol pk.VarInt, ip string) {
	var p pk.Packet
	for i := 0; i < 2; i++ {
		conn.ReadPacket(&p)
		switch p.ID {
		case packetid.StatusRequest:
			server.Logger.Debug("[TCP] (["+ip+"]", "-> Server) Sent StatusRequest packet")
			max := server.Config.MaxPlayers
			if max == -1 {
				max = len(server.Players) + 1
			}
			players := make([]Player, 0)
			for _, player := range server.Players {
				players = append(players, player)
			}
			response := StatusResponse{
				Version: Version{
					Name:     "GoCraftServer",
					Protocol: int(Protocol),
				},
				Players: Players{
					Max:    max,
					Online: len(server.Players),
					Sample: players,
				},
				Description: Description{
					Text: server.Config.MOTD,
				},
				EnforcesSecureChat: true,
				PreviewsChat:       true,
			}
			if server.Config.Icon.Enable {
				data, err := os.ReadFile(server.Config.Icon.Path)
				if err != nil {
					server.Logger.Warn("Server icon is enabled but wasn't found; ignoring")
				} else {
					image, format, _ := image.DecodeConfig(bytes.NewReader(data))
					if format == "png" {
						if image.Width == 64 && image.Height == 64 {
							icon := base64.StdEncoding.EncodeToString(data)
							response.Favicon = fmt.Sprintf("data:image/png;base64,%s", icon)
						} else {
							server.Logger.Debug("Server icon is not a 64x64 png file; ignoring")
						}
					} else {
						server.Logger.Debug("Server icon is not a 64x64 png file; ignoring")
					}
				}
			}
			conn.WritePacket(pk.Marshal(0x00, pk.String(CreateStatusResponse(response))))
			server.Logger.Debug("[TCP] (Server -> ["+ip+"])", "Sent StatusResponse packet")
		case packetid.StatusPingRequest:
			server.Logger.Debug("[TCP] (["+ip+"]", "-> Server) Sent StatusPingRequest packet")
			conn.WritePacket(p)
			server.Logger.Debug("[TCP] (Server -> ["+ip+"])", "Sent StatusPongResponse packet")
		}
	}
}

const (
	PlayerInfoAddPlayer = iota
	PlayerInfoInitializeChat
	PlayerInfoUpdateGameMode
	PlayerInfoUpdateListed
	PlayerInfoUpdateLatency
	PlayerInfoUpdateDisplayName
	PlayerInfoEnumGuard
)

func NewPlayerInfoAction(actions ...int) pk.FixedBitSet {
	enumSet := pk.NewFixedBitSet(PlayerInfoEnumGuard)
	for _, action := range actions {
		enumSet.Set(action, true)
	}
	return enumSet
}
