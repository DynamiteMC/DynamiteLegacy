package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"

	r "math/rand"
	"time"

	"encoding/binary"
	"fmt"

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
	"github.com/Tnze/go-mc/registry"
	"github.com/Tnze/go-mc/server/auth"
	"github.com/Tnze/go-mc/yggdrasil/user"
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

func getNetworkRegistry(protocol pk.VarInt) (reg registry.NetworkCodec) {
	data, _ := registries.ReadFile("registry.nbt")
	nbt.Unmarshal(data, &reg)
	return
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

func TCPListen() {
	var err error
	server.TCPListener, err = net.ListenMC(server.Config.ServerIP + ":" + fmt.Sprint(server.Config.ServerPort))
	if err != nil {
		server.Logger.Error("[TCP] Failed to listen: %s", err.Error())
		os.Exit(1)
	}

	server.Logger.Info("[TCP] Listening on %s:%d", server.Config.ServerIP, server.Config.ServerPort)
}

func HandleTCPRequest(conn net.Conn) {
	defer conn.Close()
	var packet pk.Packet
	conn.ReadPacket(&packet)
	ip := conn.Socket.RemoteAddr().String()
	if packet.ID == 0x00 { // 1.7+
		server.Logger.Debug("[TCP] ([%s] -> Server) Sent handshake", ip)
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
		case packetid.StatusPingRequest:
			handleTCPPing(conn, Protocol, ip) // Ping
		case 2:
			{ // login
				if 762 > int(Protocol) {
					conn.WritePacket(pk.Marshal(packetid.LoginDisconnect, chat.Text(server.Config.Messages.ProtocolOld)))
					conn.Close()
					return
				}
				if 762 < int(Protocol) {
					conn.WritePacket(pk.Marshal(packetid.LoginDisconnect, chat.Text(server.Config.Messages.ProtocolNew)))
					conn.Close()
					return
				}
				var p pk.Packet
				conn.ReadPacket(&p)
				var (
					name pk.String
					uuid pk.UUID
				)
				p.Scan(&name, &uuid)
				server.Logger.Debug("[TCP] ([%s] -> Server) Sent LoginStart packet. Username: %s", ip, name)
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
				server.Logger.Info("[%s] Player %s (%s) is attempting to join", ip, name, idString)
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
					server.Logger.Info("[%s] Player %s (%s) attempt failed. reason: %s", ip, name, idString, reason)
					conn.WritePacket(pk.Marshal(packetid.LoginDisconnect, r))
				}
				conn.WritePacket(pk.Marshal(
					packetid.LoginSuccess,
					id,
					pk.String(name),
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
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLogin,
					pk.Int(server.NewEntityID()),
					pk.Boolean(server.Config.Hardcore),
					pk.UnsignedByte(gamemode),
					pk.Byte(-1),
					pk.Array([]pk.Identifier{
						pk.Identifier("world"),
					}),
					pk.NBT(getNetworkRegistry(Protocol)),
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
				))
				conn.WritePacket(pk.Marshal(packetid.ClientboundSetDefaultSpawnPosition,
					pk.Position{X: 0, Y: 0, Z: 0},
					pk.Float(50)))
				var lastKeepAliveId int
				player := Player{
					Name: fmt.Sprint(name),
					UUID: UUID{
						String: idString,
						Binary: id,
					},
					Connection: &conn,
					Properties: properties,
					IP:         ip,
				}
				for {
					var packet pk.Packet
					err := conn.ReadPacket(&packet)
					if err != nil {
						server.Logger.Info("[%s] Player %s (%s) disconnected", ip, name, idString)
						server.Events.Emit("PlayerLeave", player)
						return
					}
					switch packet.ID {
					case int32(packetid.ServerboundClientInformation):
						{
							server.Logger.Info("[%s] Player %s (%s) joined the server", ip, name, idString)
							server.Players[idString] = player
							server.PlayerNames[fmt.Sprint(name)] = idString
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
							go func() {
								for range ticker.C {
									lastKeepAliveId = r.Intn(1000)
									conn.WritePacket(pk.Marshal(packetid.ClientboundKeepAlive, pk.Long(lastKeepAliveId)))
									server.Logger.Debug("[TCP] (Server -> [%s]) Sent KeepAlive packet", ip)
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
							server.Logger.Debug("[TCP] ([%s] -> Server) Sent KeepAlive packet", ip)
						}
					case int32(packetid.ServerboundChatCommand):
						{
							var command pk.String

							packet.Scan(&command)
							server.Events.Emit("PlayerCommand", player, command)
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
							var reason pk.VarInt
							packet.Scan(&reason)
							fmt.Println(reason)
							if reason == 0 {
								server.Logger.Info("[%s] Player %s (%s) disconnected", ip, name, idString)
								conn.Close()
								server.Events.Emit("PlayerLeave", player)
								return
							}
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
			server.Logger.Debug("[TCP] ([%s] -> Server) Sent StatusRequest packet", ip)
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
					Name:     "GoCraft 1.19.4",
					Protocol: 762,
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
				success, code, data := server.GetFavicon()
				if !success {
					switch code {
					case FAVICON_NOTFOUND:
						{
							server.Logger.Warn("Server icon is enabled but wasn't found; ignoring")
						}
					case FAVICON_INVALID_FORMAT, FAVICON_INVALID_DIMENSIONS:
						{
							server.Logger.Debug("Server icon is not a 64x64 png file; ignoring")
						}
					}
				} else {
					icon := base64.StdEncoding.EncodeToString(data)
					response.Favicon = fmt.Sprintf("data:image/png;base64,%s", icon)
				}
			}
			conn.WritePacket(pk.Marshal(0x00, pk.String(CreateStatusResponse(response))))
			server.Logger.Debug("[TCP] (Server -> [%s]) Sent StatusResponse packet", ip)
		case packetid.StatusPingRequest:
			server.Logger.Debug("[TCP] ([%s] -> Server) Sent StatusPingRequest packet", ip)
			conn.WritePacket(p)
			server.Logger.Debug("[TCP] (Server -> [%s]) Sent StatusPongResponse packet", ip)
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
