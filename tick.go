package main

import (
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
	if n%8 == 0 {
		world.SubtickChunkLoad()
	}
}

func (world World) SubtickChunkLoad() {
	for _, p := range server.Players {
		if p.World != world.Name {
			continue
		}
		x := int32(p.Position[0]) >> 4
		y := int32(p.Position[1]) >> 4
		z := int32(p.Position[2]) >> 4
		if newChunkPos := [3]int32{x, y, z}; newChunkPos != p.ChunkPos {
			p.ChunkPos = newChunkPos
			p.Connection.WritePacket(pk.Marshal(packetid.ClientboundSetChunkCacheCenter, pk.VarInt(x), pk.VarInt(z)))
		}
	}
LoadChunk:
	for _, player := range server.Players {
		if player.World != world.Name {
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
			player.Connection.WritePacket(pk.Marshal(packetid.ClientboundLevelChunkWithLight, level.ChunkPos(pos), lc))
		}
	}
	/*for viewer, loader := range w.loaders {
		loader.calcUnusedChunks()
		for _, pos := range loader.unloadQueue {
			delete(loader.loaded, pos)
			if !w.chunks[pos].RemoveViewer(viewer) {
				w.log.Panic("viewer is not found in the loaded chunk")
			}
			viewer.ViewChunkUnload(pos)
		}
	}
	var unloadQueue [][2]int32
	for pos, chunk := range world.Chunks {
		if len(chunk.viewers) == 0 {
			unloadQueue = append(unloadQueue, pos)
		}
	}
	for i := range unloadQueue {
		w.unloadChunk(unloadQueue[i])
	}*/
}
