package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"image"

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

func handleTCPRequest(conn net.Conn) {
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
				var p pk.Packet
				for i := 0; i < 2; i++ {
					conn.ReadPacket(&p)
					switch p.ID {
					case packetid.StatusRequest:
						logger.Debug("[TCP] (["+ip+"]", "-> Server) Sent StatusRequest packet")
						response := StatusResponse{
							Version: Version{
								Name:     "GoCraftServer",
								Protocol: int(Protocol),
							},
							Players: Players{
								Max:    config.MaxPlayers,
								Online: len(server.Players),
								Sample: server.Players,
							},
							Description: Description{
								Text: config.MOTD,
							},
							EnforcesSecureChat: true,
							PreviewsChat:       true,
						}
						if config.Icon.Enable {
							data, err := os.ReadFile(config.Icon.Path)
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
				if config.Online {
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
							reasonNice = config.Messages.NotInWhitelist
						}
					case 2:
						{
							reason = "player is banned"
							reasonNice = config.Messages.Banned
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
			}
		}
	} else if packet.ID == 122 { //1.6-
	}
}
