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

	"github.com/go-mc/server/world"

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
)

type MojangLoginHandler struct {
	privateKey     atomic.Pointer[rsa.PrivateKey]
	lockPrivateKey sync.Mutex
}

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
		logger.Debug("[TCP] (["+ip+"]", "-> Server) Sent handshake")
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
				var p pk.Packet
				conn.ReadPacket(&p)
				var (
					name pk.String
					uuid pk.UUID
				)
				p.Scan(&name, &uuid)
				logger.Debug("[TCP] (["+ip+"]", "-> Server) Sent LoginStart packet. Username:", name)
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
				logger.Info("["+ip+"]", "Player", name, "("+idString+")", "is attempting to join.")
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
					logger.Info("["+ip+"]", "Player", name, "("+idString+")", "attempt failed. reason:", reason)
					conn.WritePacket(pk.Marshal(packetid.LoginDisconnect, r))
				}
				conn.WritePacket(pk.Marshal(
					packetid.LoginSuccess,
					id,
					pk.String(name),
					pk.Array(properties),
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
				if Protocol >= 763 {
					fields = append(fields, pk.VarInt(3))
				}
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLogin,
					fields...,
				))
				conn.WritePacket(pk.Marshal(packetid.ClientboundSetDefaultSpawnPosition,
					pk.Position{X: 100, Y: 100, Z: 100},
					pk.Float(50)))
				/*conn.WritePacket(pk.Marshal(0x3C,
					pk.Double(100),
					pk.Double(100),
					pk.Double(100),
					pk.Float(12),
					pk.Float(12),
					pk.Byte(0),
					pk.VarInt(0),
				))*/
				data, _ := os.ReadFile("heightmap.nbt")
				var d = make(map[string]interface{})
				nbt.NewDecoder(bytes.NewReader(data)).Decode(d)
				/*conn.WritePacket(pk.Marshal(
					packetid.ClientboundLevelChunkWithLight,
					pk.Int(0),
					pk.Int(0),
					pk.NBT(d),
					pk.VarInt(0),
					pk.ByteArray{},
				))*/
				var lastKeepAliveId int
				//var lastPacket pk.Packet
				player := Player{
					Name:       fmt.Sprint(name),
					UUID:       idString,
					UUIDb:      id,
					Connection: conn,
					Properties: properties,
				}
				var (
					joined = false
				)
				for {
					var packet pk.Packet
					conn.ReadPacket(&packet)
					//lastPacket = packet
					switch packet.ID {
					case 8:
						{
							logger.Info("["+ip+"]", "Player", name, "("+idString+")", "joined the server")
							server.Players[idString] = player
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
									logger.Debug("[TCP] (Server -> ["+ip+"])", "Sent KeepAlive packet")
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
							logger.Debug("[TCP] (["+ip+"]", "-> Server) Sent KeepAlive packet")
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

							conn.Close()
							if joined {
								server.Events.Emit("PlayerLeave", player)
							}
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
			logger.Debug("[TCP] (["+ip+"]", "-> Server) Sent StatusRequest packet")
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
					logger.Warn("Server icon is enabled but wasn't found; ignoring")
				} else {
					image, format, _ := image.DecodeConfig(bytes.NewReader(data))
					if format == "png" {
						if image.Width == 64 && image.Height == 64 {
							icon := base64.StdEncoding.EncodeToString(data)
							response.Favicon = fmt.Sprintf("data:image/png;base64,%s", icon)
						} else {
							logger.Debug("Server icon is not a 64x64 png file; ignoring")
						}
					} else {
						logger.Debug("Server icon is not a 64x64 png file; ignoring")
					}
				}
			}
			conn.WritePacket(pk.Marshal(0x00, pk.String(CreateStatusResponse(response))))
			logger.Debug("[TCP] (Server -> ["+ip+"])", "Sent StatusResponse packet")
		case packetid.StatusPingRequest:
			logger.Debug("[TCP] (["+ip+"]", "-> Server) Sent StatusPingRequest packet")
			conn.WritePacket(p)
			logger.Debug("[TCP] (Server -> ["+ip+"])", "Sent StatusPongResponse packet")
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
