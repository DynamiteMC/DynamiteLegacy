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
					r := chat.Message{Text: reasonNice}
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
				conn.WritePacket(pk.Marshal(
					packetid.ClientboundLogin,
					pk.Int(0),
					pk.Boolean(server.Config.Hardcore),
					pk.UnsignedByte(gamemode),
					pk.Byte(-1),
					pk.Array([]pk.Identifier{
						pk.Identifier("world"),
					}),
					pk.NBT(world.NetworkCodec),
					pk.Identifier("minecraft:overworld"),
					pk.Identifier("world"),
					pk.Long(binary.BigEndian.Uint64([]byte("e53e40231b931de13ba973e5154cd572ad7d001e2bf5f7d6e26e0ae48252653f")[:8])),
					pk.VarInt(server.Config.MaxPlayers), // Max players (ignored by client)
					pk.VarInt(12),                       // View Distance
					pk.VarInt(12),                       // Simulation Distance
					pk.Boolean(false),                   // Reduced Debug Info
					pk.Boolean(false),                   // Enable respawn screen
					pk.Boolean(false),                   // Is Debug
					pk.Boolean(false),                   // Is Flat
					pk.Boolean(false),                   // Has Last Death Location
				))
				conn.WritePacket(pk.Marshal(0x50,
					pk.Position{X: 100, Y: 100, Z: 100},
					pk.Float(50)))
				logger.Info("["+ip+"]", "Player", name, "("+idString+")", "joined the server")
				player := Player{
					Name:       fmt.Sprint(name),
					UUID:       idString,
					UUIDb:      id,
					Connection: conn,
					Properties: properties,
				}
				server.Players[idString] = player
				lastPacketId := p.ID
				lastServerKeepAlive := time.Now()
				var lastKeepAliveId int
				server.Events.Emit("PlayerJoin", player)
				go func() {
					for {
						if server.Players[idString].UUID != idString {
							break
						}
						if time.Since(lastServerKeepAlive).Seconds() >= 10 {
							lastKeepAliveId = r.Intn(1000)
							conn.WritePacket(pk.Marshal(packetid.ServerboundKeepAlive, pk.Long(lastKeepAliveId)))
							logger.Debug("[TCP] (Server -> ["+ip+"])", "Sent KeepAlive packet")
							lastServerKeepAlive = time.Now()
						}
					}
				}()
				for {
					var packet pk.Packet
					conn.ReadPacket(&packet)
					if lastPacketId == packet.ID {
						continue
					}
					if packet.ID == int32(packetid.ClientboundKeepAlive) {
						var id pk.Long
						packet.Scan(&id)
						if id != pk.Long(lastKeepAliveId) {
							conn.Close()
							server.Events.Emit("PlayerLeave", player)
							break
						}
						logger.Debug("[TCP] (["+ip+"]", "-> Server) Sent KeepAlive packet")
					}
					if packet.ID == 0 {
						server.Events.Emit("PlayerLeave", player)
					}
					lastPacketId = packet.ID
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
