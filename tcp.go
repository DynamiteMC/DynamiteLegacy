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

func getNetworkRegistry() (reg registry.NetworkCodec) {
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
	server.Listener, err = net.ListenMC(server.Config.ServerIP + ":" + fmt.Sprint(server.Config.ServerPort))
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
			if PROTOCOL_1_19_4 > int(Protocol) {
				conn.WritePacket(pk.Marshal(packetid.LoginDisconnect, chat.Text(server.Config.Messages.ProtocolOld)))
				conn.Close()
				return
			}
			if PROTOCOL_1_19_4 < int(Protocol) {
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
			d := MojangLoginHandler{}
			var serverKey *rsa.PrivateKey
			serverKey, err = d.getPrivateKey()
			if err != nil {
				return
			}
			var resp *auth.Resp
			resp, err = auth.Encrypt(&conn, fmt.Sprint(name), serverKey)
			if err != nil {
				if server.Config.Online {
					conn.WritePacket(pk.Marshal(packetid.LoginDisconnect, chat.Text(server.Config.Messages.OnlineMode)))
					return
				} else {
					id = pk.UUID(offline.NameToUUID(string(name)))
					idString = fmt.Sprint(offline.NameToUUID(string(name)))
				}
			} else {
				name = pk.String(resp.Name)
				idString = fmt.Sprint(resp.ID)
				id = pk.UUID(resp.ID)
				properties = resp.Properties
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
			var dimensions []pk.Identifier
			for name := range server.Worlds {
				dimensions = append(dimensions, pk.Identifier(name))
			}
			data := server.GetPlayerData(idString)
			if data == nil {
				data = &PlayerData{
					Attributes:       []interface{}{},
					OnGround:         1,
					Health:           20,
					Dimension:        string(dimensions[0]),
					Fire:             -20,
					Score:            0,
					SelectedItemSlot: 0,
					EnderItems:       []interface{}{},
					Inventory:        []InventorySlot{},
					Pos: []float64{
						float64(server.Level.Data.SpawnX),
						float64(server.Level.Data.SpawnY),
						float64(server.Level.Data.SpawnZ),
					},
					Motion: []interface{}{
						float64(0),
						float64(0),
						float64(0),
					},
					Rotation: []float32{
						90,
						90,
					},
					XpLevel:             0,
					XpTotal:             0,
					XpP:                 0,
					DeathTime:           0,
					HurtTime:            0,
					SleepTimer:          0,
					SeenCredits:         0,
					PlayerGameType:      1,
					FoodLevel:           20,
					FoodExhaustionLevel: 0,
					FoodSaturationLevel: 5,
					FoodTickTimer:       0,
					RecipeBook: PlayerDataRecipeBook{
						IsBlastingFurnaceFilteringCraftable: 0,
						IsBlastingFurnaceGuiOpen:            0,
						IsFilteringCraftable:                0,
						IsFurnaceFilteringCraftable:         0,
						IsFurnaceGuiOpen:                    0,
						IsGuiOpen:                           0,
						IsSmokerFilteringCraftables:         0,
						IsSmokerGuiOpen:                     0,
						Recipes:                             []interface{}{},
						ToBeDisplayed:                       []interface{}{},
					},
				}
				server.WritePlayerData(idString, *data)
			}
			entityId := server.NewEntityID()
			conn.WritePacket(pk.Marshal(
				packetid.ClientboundLogin,
				pk.Int(entityId),
				pk.Boolean(server.Config.Hardcore),
				pk.UnsignedByte(gamemode),
				pk.Byte(-1),
				pk.Array(dimensions),
				pk.NBT(getNetworkRegistry()),
				pk.Identifier("minecraft:overworld"),
				pk.Identifier(data.Dimension),
				pk.Long(binary.BigEndian.Uint64(hashedSeed[:8])),
				pk.VarInt(server.Config.MaxPlayers),
				pk.VarInt(server.Config.ViewDistance),
				pk.VarInt(server.Config.SimulationDistance),
				pk.Boolean(false),
				pk.Boolean(false),
				pk.Boolean(false),
				pk.Boolean(false),
				pk.Boolean(false),
			))
			conn.WritePacket(pk.Marshal(packetid.ClientboundPlayerPosition,
				pk.Double(data.Pos[0]),     //x
				pk.Double(data.Pos[1]),     //y
				pk.Double(data.Pos[2]),     //z
				pk.Float(data.Rotation[0]), //yaw
				pk.Float(data.Rotation[1]), //pitch
				pk.Byte(0),
				pk.VarInt(server.NewTeleportID()),
			))
			conn.WritePacket(pk.Marshal(packetid.ClientboundSetDefaultSpawnPosition,
				pk.Position{X: int(server.Level.Data.SpawnX), Y: int(server.Level.Data.SpawnY), Z: int(server.Level.Data.SpawnZ)},
				pk.Float(0)))
			if len(data.Inventory) > 0 {
				conn.WritePacket(pk.Marshal(packetid.ClientboundContainerSetContent,
					pk.UnsignedByte(0),
					pk.VarInt(0),
					data,
				))
			}
			var lastKeepAliveId int
			player := &Player{
				Name: fmt.Sprint(name),
				UUID: UUID{
					String: idString,
					Binary: id,
				},
				Connection:   &conn,
				Properties:   properties,
				IP:           ip,
				LoadedChunks: make(map[[2]int32]struct{}),
				Data:         *data,
				EntityID:     entityId,
				OldPosition:  [3]int32{-1, -1, -1},
			}
			joined := false
			for {
				var packet pk.Packet
				err := conn.ReadPacket(&packet)
				if err != nil {
					server.Worlds[player.Data.Dimension].TickLock.Lock()
					for pos := range player.LoadedChunks {
						server.Worlds[player.Data.Dimension].Chunks[pos].RemoveViewer(player.UUID.String)
					}
					server.Worlds[player.Data.Dimension].TickLock.Unlock()
					server.Logger.Info("[%s] Player %s (%s) disconnected", ip, name, idString)
					server.Events.Emit("PlayerLeave", player)
					return
				}
				switch packet.ID {
				case int32(packetid.ServerboundClientInformation):
					{
						packet.Scan(&player.Client.Locale,
							&player.Client.ViewDistance,
							&player.Client.ChatMode,
							&player.Client.ChatColors,
							&player.Client.DisplayedSkinParts,
							&player.Client.MainHand,
							&player.Client.EnableTextFiltering,
							&player.Client.AllowServerListings,
						)
						if joined {
							continue
						}
						server.Players.Lock()
						server.Players.Players[idString] = player
						server.Players.PlayerNames[fmt.Sprint(name)] = idString
						server.Players.PlayerIDs = append(server.Players.PlayerIDs, idString)
						server.Players.Unlock()
						joined = true

						server.Logger.Info("[%s] Player %s (%s) joined the server", ip, name, idString)
						server.Events.Emit("PlayerJoin", player, conn)
						ticker := time.NewTicker(15 * time.Second)
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
				case int32(packetid.ServerboundMovePlayerPos):
					{
						var (
							x pk.Double
							y pk.Double
							z pk.Double
						)
						packet.Scan(&x, &y, &z)
						if player.Position == [3]int32{int32(x), int32(y), int32(z)} {
							continue
						}
						player.OldPosition = player.Position
						player.Position = [3]int32{int32(x), int32(y), int32(z)}
					}
				case int32(packetid.ServerboundMovePlayerPosRot):
					{
						var (
							x     pk.Double
							y     pk.Double
							z     pk.Double
							yaw   pk.Float
							pitch pk.Float
						)
						packet.Scan(&x, &y, &z, &yaw, &pitch)
						if player.Position == [3]int32{int32(x), int32(y), int32(z)} {
							continue
						}
						player.OldPosition = player.Position
						player.Position = [3]int32{int32(x), int32(y), int32(z)}
					}
				case int32(packetid.ServerboundMovePlayerRot):
					{
						var (
							yaw   pk.Float
							pitch pk.Float
						)
						packet.Scan(&yaw, &pitch)
						player.Rotation = [2]float32{float32(yaw), float32(pitch)}
					}
				case int32(packetid.ServerboundChat):
					{
						server.Events.Emit("PlayerChatMessage", player, packet)
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
					}
				}
			}
		}
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
				max = len(server.Players.Players) + 1
			}
			players := server.Players.AsBase()
			response := StatusResponse{
				Version: Version{
					Name:     "GoCraft 1.19.4",
					Protocol: PROTOCOL_1_19_4,
				},
				Players: Players{
					Max:    max,
					Online: len(players),
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
