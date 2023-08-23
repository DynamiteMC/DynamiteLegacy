package main

import (
	"fmt"
	"time"

	"github.com/Tnze/go-mc/data/packetid"
	"github.com/Tnze/go-mc/level"
	pk "github.com/Tnze/go-mc/net/packet"
)

func (world World) TickLoop() {
	var n uint
	for range time.Tick(time.Microsecond * 20) {
		world.Tick(n)
		n++
	}
}

func (world World) Tick(n uint) {
	world.TickLock.Lock()
	defer world.TickLock.Unlock()
	if n%8 == 0 {
		world.SubtickChunkLoad(n)
	}
	//world.TickPlayerUpdate(n)
}

/*func (world World) TickPlayerUpdate(tick uint) {
	for _, p := range server.Players.Players {
		if p.Data.Dimension != world.Name {
			continue
		}
		x := int32(p.Position[0])
		y := int32(p.Position[1])
		z := int32(p.Position[2])
		if p.Position != p.OldPosition {
			server.BroadcastPacketExcept(pk.Marshal(packetid.ClientboundMoveEntityPos,
				pk.VarInt(p.EntityID),
				pk.Short((x*32-p.OldPosition[0]*32)*128),
				pk.Short((y*32-p.OldPosition[1]*32)*128),
				pk.Short((z*32-p.OldPosition[2]*32)*128),
				pk.Boolean(true),
			), p.UUID.String)
		}
		p.LastTick = tick
	}
}*/

func (world World) SubtickChunkLoad(tick uint) {
	for _, p := range server.Players.Players {
		if p.Data.Dimension != world.Name {
			continue
		}
		x := int32(p.Position[0]) >> 4
		y := int32(p.Position[1]) >> 4
		z := int32(p.Position[2]) >> 4
		if newChunkPos := [3]int32{x, y, z}; newChunkPos != p.ChunkPos {
			p.ChunkPos = newChunkPos
			p.Connection.WritePacket(pk.Marshal(packetid.ClientboundSetChunkCacheCenter, pk.VarInt(x), pk.VarInt(z)))
			fmt.Println("sent packet 78 for player", p.Name)
		}
		p.LastTick = tick
	}
LoadChunk:
	for _, player := range server.Players.Players {
		if player.Data.Dimension != world.Name {
			continue
		}
		player.CalculateLoadingQueue()
		for _, pos := range player.LoadQueue {
			if _, ok := world.Chunks[pos]; !ok {
				if !world.LoadChunk(pos) {
					break LoadChunk
				}
			}
			lc := world.Chunks[pos]
			if lc == nil {
				break LoadChunk
			}
			player.LoadedChunks[pos] = struct{}{}
			lc.AddViewer(player.UUID.String)
			lc.Lock()
			player.Connection.WritePacket(pk.Marshal(packetid.ClientboundLevelChunkWithLight, level.ChunkPos(pos), lc.Chunk))
			fmt.Println("sent packet 36 for player", player.Name, pos, len(lc.Chunk.Sections))
			lc.Unlock()
		}
	}
	for _, player := range server.Players.Players {
		if player.Data.Dimension != world.Name {
			continue
		}
		player.CalculateUnusedChunks()
		for _, pos := range player.UnloadQueue {
			delete(player.LoadedChunks, pos)
			world.Chunks[pos].RemoveViewer(player.UUID.String)
			player.Connection.WritePacket(pk.Marshal(packetid.ClientboundForgetLevelChunk, level.ChunkPos(pos)))
			fmt.Println("sent packet 30 for player", player.Name)
		}
	}
	var unloadQueue [][2]int32
	for pos, chunk := range world.Chunks {
		if len(chunk.Viewers) == 0 {
			unloadQueue = append(unloadQueue, pos)
		}
	}
	for i := range unloadQueue {
		world.UnloadChunk(unloadQueue[i])
	}
}
