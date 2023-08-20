package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"strconv"

	"github.com/Tnze/go-mc/level/biome"
	"github.com/Tnze/go-mc/level/block"
	"github.com/Tnze/go-mc/nbt"
	pk "github.com/Tnze/go-mc/net/packet"
	"github.com/Tnze/go-mc/save"
)

type State interface {
	~int
}
type (
	BlocksState = block.StateID
	BiomesState = biome.Type
)

type PaletteContainer[T State] struct {
	bits    int
	config  paletteCfg[T]
	palette palette[T]
	data    *BitStorage
}

func NewStatesPaletteContainer(length int, defaultValue BlocksState) *PaletteContainer[BlocksState] {
	return &PaletteContainer[BlocksState]{
		bits:    0,
		config:  statesCfg{},
		palette: &singleValuePalette[BlocksState]{v: defaultValue},
		data:    NewBitStorage(0, length, nil),
	}
}

func NewStatesPaletteContainerWithData(length int, data []uint64, pat []BlocksState) *PaletteContainer[BlocksState] {
	var p palette[BlocksState]
	n := calcBitsPerValue(length, len(data))
	switch n {
	case 0:
		p = &singleValuePalette[BlocksState]{pat[0]}
	case 1, 2, 3, 4:
		n = 4
		p = &linearPalette[BlocksState]{
			values: pat,
			bits:   n,
		}
	case 5, 6, 7, 8:
		ids := make(map[BlocksState]int)
		for i, v := range pat {
			ids[v] = i
		}
		p = &hashPalette[BlocksState]{
			ids:    ids,
			values: pat,
			bits:   n,
		}
	default:
		p = &globalPalette[BlocksState]{}
	}
	return &PaletteContainer[BlocksState]{
		bits:    n,
		config:  statesCfg{},
		palette: p,
		data:    NewBitStorage(n, length, data),
	}
}

func NewBiomesPaletteContainer(length int, defaultValue BiomesState) *PaletteContainer[BiomesState] {
	return &PaletteContainer[BiomesState]{
		bits:    0,
		config:  biomesCfg{},
		palette: &singleValuePalette[BiomesState]{v: defaultValue},
		data:    NewBitStorage(0, length, nil),
	}
}

func NewBiomesPaletteContainerWithData(length int, data []uint64, pat []BiomesState) *PaletteContainer[BiomesState] {
	var p palette[BiomesState]
	n := calcBitsPerValue(length, len(data))
	switch n {
	case 0:
		p = &singleValuePalette[BiomesState]{pat[0]}
	case 1, 2, 3:
		p = &linearPalette[BiomesState]{
			values: pat,
			bits:   n,
		}
	default:
		p = &globalPalette[BiomesState]{}
	}
	return &PaletteContainer[BiomesState]{
		bits:    n,
		config:  biomesCfg{},
		palette: p,
		data:    NewBitStorage(n, length, data),
	}
}

func (p *PaletteContainer[T]) Get(i int) T {
	return p.palette.value(p.data.Get(i))
}

func (p *PaletteContainer[T]) Set(i int, v T) {
	if vv, ok := p.palette.id(v); ok {
		p.data.Set(i, vv)
	} else {
		length := p.data.Len()
		// resize
		newContainer := PaletteContainer[T]{
			bits:    vv,
			config:  p.config,
			palette: p.config.create(vv),
			data:    NewBitStorage(p.config.bits(vv), length, nil),
		}
		// copy
		for i := 0; i < length; i++ {
			newContainer.Set(i, p.Get(i))
		}

		if vv, ok := newContainer.palette.id(v); !ok {
			panic("not reachable")
		} else {
			newContainer.data.Set(i, vv)
		}
		*p = newContainer
	}
}

func (p *PaletteContainer[T]) ReadFrom(r io.Reader) (n int64, err error) {
	var nBits pk.UnsignedByte
	n, err = nBits.ReadFrom(r)
	if err != nil {
		return
	}
	p.bits = p.config.bits(int(nBits))
	p.palette = p.config.create(int(nBits))

	nn, err := p.palette.ReadFrom(r)
	n += nn
	if err != nil {
		return n, err
	}

	nn, err = p.data.ReadFrom(r)
	n += nn
	if err != nil {
		return n, err
	}
	return n, p.data.Fix(p.bits)
}

func (p *PaletteContainer[T]) WriteTo(w io.Writer) (n int64, err error) {
	return pk.Tuple{
		pk.UnsignedByte(p.bits),
		p.palette,
		p.data,
	}.WriteTo(w)
}

// Palette export the raw palette values for @maxsupermanhd.
// Others shouldn't call this because this might be removed
// after max doesn't need it anymore.
func (p *PaletteContainer[T]) Palette() []T {
	return p.palette.export()
}

type paletteCfg[T State] interface {
	bits(int) int
	create(bits int) palette[T]
}

type statesCfg struct{}

func (s statesCfg) bits(bits int) int {
	switch bits {
	case 0:
		return 0
	case 1, 2, 3, 4:
		return 4
	case 5, 6, 7, 8:
		return bits
	default:
		return block.BitsPerBlock
	}
}

func (s statesCfg) create(bits int) palette[BlocksState] {
	switch bits {
	case 0:
		return &singleValuePalette[BlocksState]{v: -1}
	case 1, 2, 3, 4:
		return &linearPalette[BlocksState]{bits: 4, values: make([]BlocksState, 0, 1<<4)}
	case 5, 6, 7, 8:
		return &hashPalette[BlocksState]{
			bits:   bits,
			ids:    make(map[BlocksState]int),
			values: make([]BlocksState, 0, 1<<bits),
		}
	default:
		return &globalPalette[BlocksState]{}
	}
}

type biomesCfg struct{}

func (b biomesCfg) bits(bits int) int {
	switch bits {
	case 0:
		return 0
	case 1, 2, 3:
		return bits
	default:
		return biome.BitsPerBiome
	}
}

func (b biomesCfg) create(bits int) palette[BiomesState] {
	switch bits {
	case 0:
		return &singleValuePalette[BiomesState]{v: -1}
	case 1, 2, 3:
		return &linearPalette[BiomesState]{bits: bits, values: make([]BiomesState, 0, 1<<bits)}
	default:
		return &globalPalette[BiomesState]{}
	}
}

type palette[T State] interface {
	pk.Field
	// id return the index of state v in the palette and true if existed.
	// otherwise return the new bits for resize and false.
	id(v T) (int, bool)
	value(i int) T
	export() []T
}

type singleValuePalette[T State] struct {
	v T
}

func (s *singleValuePalette[T]) id(v T) (int, bool) {
	if s.v == v {
		return 0, true
	}
	// We have 2 values now. At least 1 bit is required.
	return 1, false
}

func (s *singleValuePalette[T]) value(i int) T {
	if i == 0 {
		return s.v
	}
	panic("singleValuePalette: " + strconv.Itoa(i) + " out of bounds")
}

func (s *singleValuePalette[T]) export() []T {
	return []T{s.v}
}

func (s *singleValuePalette[T]) ReadFrom(r io.Reader) (n int64, err error) {
	var i pk.VarInt
	n, err = i.ReadFrom(r)
	if err != nil {
		return
	}
	s.v = T(i)
	return
}

func (s *singleValuePalette[T]) WriteTo(w io.Writer) (n int64, err error) {
	return pk.VarInt(s.v).WriteTo(w)
}

type linearPalette[T State] struct {
	values []T
	bits   int
}

func (l *linearPalette[T]) id(v T) (int, bool) {
	for i, t := range l.values {
		if t == v {
			return i, true
		}
	}
	if cap(l.values)-len(l.values) > 0 {
		l.values = append(l.values, v)
		return len(l.values) - 1, true
	}
	return l.bits + 1, false
}

func (l *linearPalette[T]) value(i int) T {
	if i >= 0 && i < len(l.values) {
		return l.values[i]
	}
	panic("linearPalette: " + strconv.Itoa(i) + " out of bounds")
}

func (l *linearPalette[T]) export() []T {
	return l.values
}

func (l *linearPalette[T]) ReadFrom(r io.Reader) (n int64, err error) {
	var size, value pk.VarInt
	if n, err = size.ReadFrom(r); err != nil {
		return
	}
	if int(size) > cap(l.values) {
		l.values = make([]T, size)
	} else {
		l.values = l.values[:size]
	}
	for i := 0; i < int(size); i++ {
		if nn, err := value.ReadFrom(r); err != nil {
			return n + nn, err
		} else {
			n += nn
		}
		l.values[i] = T(value)
	}
	return
}

func (l *linearPalette[T]) WriteTo(w io.Writer) (n int64, err error) {
	if n, err = pk.VarInt(len(l.values)).WriteTo(w); err != nil {
		return
	}
	for _, v := range l.values {
		if nn, err := pk.VarInt(v).WriteTo(w); err != nil {
			return n + nn, err
		} else {
			n += nn
		}
	}
	return
}

type hashPalette[T State] struct {
	ids    map[T]int
	values []T
	bits   int
}

func (h *hashPalette[T]) id(v T) (int, bool) {
	if i, ok := h.ids[v]; ok {
		return i, true
	}
	if cap(h.values)-len(h.values) > 0 {
		h.ids[v] = len(h.values)
		h.values = append(h.values, v)
		return len(h.values) - 1, true
	}
	return h.bits + 1, false
}

func (h *hashPalette[T]) value(i int) T {
	if i >= 0 && i < len(h.values) {
		return h.values[i]
	}
	panic("hashPalette: " + strconv.Itoa(i) + " out of bounds")
}

func (h *hashPalette[T]) export() []T {
	return h.values
}

func (h *hashPalette[T]) ReadFrom(r io.Reader) (n int64, err error) {
	var size, value pk.VarInt
	if n, err = size.ReadFrom(r); err != nil {
		return
	}
	if int(size) > cap(h.values) {
		h.values = make([]T, size)
	} else {
		h.values = h.values[:size]
	}
	for i := 0; i < int(size); i++ {
		if nn, err := value.ReadFrom(r); err != nil {
			return n + nn, err
		} else {
			n += nn
		}
		h.values[i] = T(value)
		h.ids[T(value)] = i
	}
	return
}

func (h *hashPalette[T]) WriteTo(w io.Writer) (n int64, err error) {
	if n, err = pk.VarInt(len(h.values)).WriteTo(w); err != nil {
		return
	}
	for _, v := range h.values {
		if nn, err := pk.VarInt(v).WriteTo(w); err != nil {
			return n + nn, err
		} else {
			n += nn
		}
	}
	return
}

type globalPalette[T State] struct{}

func (g *globalPalette[T]) id(v T) (int, bool) {
	return int(v), true
}

func (g *globalPalette[T]) value(i int) T {
	return T(i)
}

func (g *globalPalette[T]) export() []T {
	return []T{}
}

func (g *globalPalette[T]) ReadFrom(_ io.Reader) (int64, error) {
	return 0, nil
}

func (g *globalPalette[T]) WriteTo(_ io.Writer) (int64, error) {
	return 0, nil
}

type ChunkStatus string

const (
	indexOutOfBounds = "index out of bounds"
	valueOutOfBounds = "value out of bounds"
)

// BitStorage implement the compacted data array used in chunk storage and heightmaps.
// You can think of this as a []intN whose N is indicated by "bits".
// For more info, see: https://wiki.vg/Chunk_Format
// This implementation is compatible with the format since Minecraft 1.16
type BitStorage struct {
	data []uint64
	mask uint64

	bits, length  int
	valuesPerLong int
}

// NewBitStorage create a new BitStorage.
//
// The "bits" is the number of bits per value, which can be calculated by math/bits.Len()
// The "length" is the number of values.
// The "data" is optional for initializing. It will panic if data != nil && len(data) != calcBitStorageSize(bits, length).
func NewBitStorage(bits, length int, data []uint64) (b *BitStorage) {
	if bits == 0 {
		return &BitStorage{
			data:          nil,
			mask:          0,
			bits:          0,
			length:        length,
			valuesPerLong: 0,
		}
	}

	b = &BitStorage{
		mask: 1<<bits - 1,
		bits: bits, length: length,
		valuesPerLong: 64 / bits,
	}
	dataLen := calcBitStorageSize(bits, length)
	b.data = make([]uint64, dataLen)
	if data != nil {
		if len(data) != dataLen {
			panic(newBitStorageErr{ArrlLen: len(data), WantLen: dataLen})
		}
		copy(b.data, data)
	}
	return
}

// calcBitStorageSize calculate how many uint64 is needed for given bits and length.
func calcBitStorageSize(bits, length int) (size int) {
	if bits == 0 {
		return 0
	}
	valuesPerLong := 64 / bits
	return (length + valuesPerLong - 1) / valuesPerLong
}

// calcBitsPerValue calculate when "longs" number of uint64 stores
// "length" number of value, how many bits are used for each value.
func calcBitsPerValue(length, longs int) (bits int) {
	if longs == 0 || length == 0 {
		return 0
	}
	valuePerLong := (length + longs - 1) / longs
	return 64 / valuePerLong
}

type newBitStorageErr struct {
	ArrlLen int
	WantLen int
}

func (i newBitStorageErr) Error() string {
	return fmt.Sprintf("invalid length given for storage, got: %d but expected: %d", i.ArrlLen, i.WantLen)
}

func (b *BitStorage) calcIndex(n int) (c, o int) {
	c = n / b.valuesPerLong
	o = (n - c*b.valuesPerLong) * b.bits
	return
}

// Swap sets v into [i], and return the previous [i] value.
func (b *BitStorage) Swap(i, v int) (old int) {
	if b.valuesPerLong == 0 {
		return 0
	}
	if v < 0 || uint64(v) > b.mask {
		panic(valueOutOfBounds)
	}
	if i < 0 || i > b.length-1 {
		panic(indexOutOfBounds)
	}
	c, offset := b.calcIndex(i)
	l := b.data[c]
	old = int(l >> offset & b.mask)
	b.data[c] = l&(b.mask<<offset^math.MaxUint64) | (uint64(v)&b.mask)<<offset
	return
}

// Set sets v into [i].
func (b *BitStorage) Set(i, v int) {
	if b.valuesPerLong == 0 {
		return
	}
	if v < 0 || uint64(v) > b.mask {
		panic(valueOutOfBounds)
	}
	if i < 0 || i > b.length-1 {
		panic(indexOutOfBounds)
	}

	c, offset := b.calcIndex(i)
	l := b.data[c]
	b.data[c] = l&(b.mask<<offset^math.MaxUint64) | (uint64(v)&b.mask)<<offset
}

// Get gets [i] value.
func (b *BitStorage) Get(i int) int {
	if b.valuesPerLong == 0 {
		return 0
	}
	if i < 0 || i > b.length-1 {
		panic(indexOutOfBounds)
	}

	c, offset := b.calcIndex(i)
	l := b.data[c]
	return int(l >> offset & b.mask)
}

// Len is the number of stored values.
func (b *BitStorage) Len() int {
	return b.length
}

// Raw return the underling array of uint64 for encoding/decoding.
func (b *BitStorage) Raw() []uint64 {
	if b == nil {
		return []uint64{}
	}
	return b.data
}

func (b *BitStorage) ReadFrom(r io.Reader) (int64, error) {
	var Len pk.VarInt
	n, err := Len.ReadFrom(r)
	if err != nil {
		return n, err
	}
	if cap(b.data) >= int(Len) {
		b.data = b.data[:Len]
	} else {
		b.data = make([]uint64, Len)
	}
	var v pk.Long
	for i := range b.data {
		nn, err := v.ReadFrom(r)
		n += nn
		if err != nil {
			return n, err
		}
		b.data[i] = uint64(v)
	}
	return n, nil
}

func (b *BitStorage) WriteTo(w io.Writer) (int64, error) {
	if b == nil {
		return pk.VarInt(0).WriteTo(w)
	}
	n, err := pk.VarInt(len(b.data)).WriteTo(w)
	if err != nil {
		return n, err
	}
	for _, v := range b.data {
		nn, err := pk.Long(v).WriteTo(w)
		n += nn
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// Fix recalculate BitStorage internal values for given bits.
// Typically, you should call this method after ReadFrom is called, which cause internal data is changed.
func (b *BitStorage) Fix(bits int) error {
	if bits == 0 {
		b.mask = 0
		b.bits = 0
		b.valuesPerLong = 0
		return nil
	}
	b.mask = 1<<bits - 1
	b.bits = bits
	b.valuesPerLong = 64 / bits
	// check data length
	dataLen := calcBitStorageSize(bits, b.length)
	if l := len(b.data); l != dataLen {
		return newBitStorageErr{ArrlLen: l, WantLen: dataLen}
	}
	return nil
}

const (
	StatusEmpty               ChunkStatus = "empty"
	StatusStructureStarts     ChunkStatus = "structure_starts"
	StatusStructureReferences ChunkStatus = "structure_references"
	StatusBiomes              ChunkStatus = "biomes"
	StatusNoise               ChunkStatus = "noise"
	StatusSurface             ChunkStatus = "surface"
	StatusCarvers             ChunkStatus = "carvers"
	StatusLiquidCarvers       ChunkStatus = "liquid_carvers"
	StatusFeatures            ChunkStatus = "features"
	StatusLight               ChunkStatus = "light"
	StatusSpawn               ChunkStatus = "spawn"
	StatusHeightmaps          ChunkStatus = "heightmaps"
	StatusFull                ChunkStatus = "full"
)

type ChunkPos [2]int32

func (c ChunkPos) WriteTo(w io.Writer) (n int64, err error) {
	n, err = pk.Int(c[0]).WriteTo(w)
	if err != nil {
		return
	}
	n1, err := pk.Int(c[1]).WriteTo(w)
	return n + n1, err
}

func (c *ChunkPos) ReadFrom(r io.Reader) (n int64, err error) {
	var x, z pk.Int
	if n, err = x.ReadFrom(r); err != nil {
		return n, err
	}
	var n1 int64
	if n1, err = z.ReadFrom(r); err != nil {
		return n + n1, err
	}
	*c = ChunkPos{int32(x), int32(z)}
	return n + n1, nil
}

type Chunk struct {
	Sections    []Section
	HeightMaps  HeightMaps
	BlockEntity []BlockEntity
	Status      ChunkStatus
	AddEdges    bool
}

func EmptyChunk(secs int) *Chunk {
	sections := make([]Section, secs)
	for i := range sections {
		sections[i] = Section{
			BlockCount: 0,
			States:     NewStatesPaletteContainer(16*16*16, 0),
			Biomes:     NewBiomesPaletteContainer(4*4*4, 0),
		}
	}
	return &Chunk{
		Sections: sections,
		HeightMaps: HeightMaps{
			WorldSurfaceWG:         NewBitStorage(bits.Len(uint(secs)*16+1), 16*16, nil),
			WorldSurface:           NewBitStorage(bits.Len(uint(secs)*16+1), 16*16, nil),
			OceanFloorWG:           NewBitStorage(bits.Len(uint(secs)*16+1), 16*16, nil),
			OceanFloor:             NewBitStorage(bits.Len(uint(secs)*16+1), 16*16, nil),
			MotionBlocking:         NewBitStorage(bits.Len(uint(secs)*16+1), 16*16, nil),
			MotionBlockingNoLeaves: NewBitStorage(bits.Len(uint(secs)*16+1), 16*16, nil),
		},
		Status: StatusEmpty,
	}
}

// ChunkFromSave convert save.Chunk to level.Chunk.
func ChunkFromSave(c *save.Chunk, addEdges bool) (*Chunk, error) {
	secs := len(c.Sections)
	sections := make([]Section, secs)
	for _, v := range c.Sections {
		i := int32(v.Y) - c.YPos
		if i < 0 || i >= int32(secs) {
			return nil, fmt.Errorf("section Y value %d out of bounds", v.Y)
		}
		var err error
		sections[i].States, err = readStatesPalette(v.BlockStates.Palette, v.BlockStates.Data)
		if err != nil {
			return nil, err
		}
		sections[i].BlockCount = countNoneAirBlocks(&sections[i])
		sections[i].Biomes, err = readBiomesPalette(v.Biomes.Palette, v.Biomes.Data)
		if err != nil {
			return nil, err
		}
		sections[i].SkyLight = v.SkyLight
		sections[i].BlockLight = v.BlockLight
	}

	blockEntities := make([]BlockEntity, len(c.BlockEntities))
	for i, v := range c.BlockEntities {
		var tmp struct {
			ID string `nbt:"id"`
			X  int32  `nbt:"x"`
			Y  int32  `nbt:"y"`
			Z  int32  `nbt:"z"`
		}
		if err := v.Unmarshal(&tmp); err != nil {
			return nil, err
		}
		blockEntities[i].Data = v
		if x, z := int(tmp.X-c.XPos<<4), int(tmp.Z-c.ZPos<<4); !blockEntities[i].PackXZ(x, z) {
			return nil, errors.New("Packing a XZ(" + strconv.Itoa(x) + ", " + strconv.Itoa(z) + ") out of bound")
		}
		blockEntities[i].Y = int16(tmp.Y)
		blockEntities[i].Type = block.EntityTypes[tmp.ID]
	}

	bitsForHeight := bits.Len( /* chunk height in blocks */ uint(secs)*16 + 1)
	return &Chunk{
		AddEdges: addEdges,
		Sections: sections,
		HeightMaps: HeightMaps{
			WorldSurface:           NewBitStorage(bitsForHeight, 16*16, c.Heightmaps["WORLD_SURFACE_WG"]),
			WorldSurfaceWG:         NewBitStorage(bitsForHeight, 16*16, c.Heightmaps["WORLD_SURFACE"]),
			OceanFloorWG:           NewBitStorage(bitsForHeight, 16*16, c.Heightmaps["OCEAN_FLOOR_WG"]),
			OceanFloor:             NewBitStorage(bitsForHeight, 16*16, c.Heightmaps["OCEAN_FLOOR"]),
			MotionBlocking:         NewBitStorage(bitsForHeight, 16*16, c.Heightmaps["MOTION_BLOCKING"]),
			MotionBlockingNoLeaves: NewBitStorage(bitsForHeight, 16*16, c.Heightmaps["MOTION_BLOCKING_NO_LEAVES"]),
		},
		BlockEntity: blockEntities,
		Status:      ChunkStatus(c.Status),
	}, nil
}

func readStatesPalette(palette []save.BlockState, data []uint64) (paletteData *PaletteContainer[BlocksState], err error) {
	statePalette := make([]BlocksState, len(palette))
	for i, v := range palette {
		b, ok := block.FromID[v.Name]
		if !ok {
			return nil, fmt.Errorf("unknown block id: %v", v.Name)
		}
		if v.Properties.Data != nil {
			if err := v.Properties.Unmarshal(&b); err != nil {
				return nil, fmt.Errorf("unmarshal block properties fail: %v", err)
			}
		}
		s, ok := block.ToStateID[b]
		if !ok {
			return nil, fmt.Errorf("unknown block: %v", b)
		}
		statePalette[i] = s
	}
	paletteData = NewStatesPaletteContainerWithData(16*16*16, data, statePalette)
	return
}

func readBiomesPalette(palette []save.BiomeState, data []uint64) (*PaletteContainer[BiomesState], error) {
	biomesRawPalette := make([]BiomesState, len(palette))
	for i, v := range palette {
		err := biomesRawPalette[i].UnmarshalText([]byte(v))
		if err != nil {
			return nil, err
		}
	}
	return NewBiomesPaletteContainerWithData(4*4*4, data, biomesRawPalette), nil
}

func countNoneAirBlocks(sec *Section) (blockCount int16) {
	for i := 0; i < 16*16*16; i++ {
		b := sec.GetBlock(i)
		if !block.IsAir(b) {
			blockCount++
		}
	}
	return
}

// ChunkToSave convert level.Chunk to save.Chunk
func ChunkToSave(c *Chunk, dst *save.Chunk) (err error) {
	secs := len(c.Sections)
	sections := make([]save.Section, secs)
	for i, v := range c.Sections {
		s := &sections[i]
		states := &s.BlockStates
		biomes := &s.Biomes
		s.Y = int8(int32(i) + dst.YPos)
		states.Palette, states.Data, err = writeStatesPalette(v.States)
		if err != nil {
			return
		}
		biomes.Palette, biomes.Data, err = writeBiomesPalette(v.Biomes)
		if err != nil {
			return
		}
		s.SkyLight = v.SkyLight
		s.BlockLight = v.BlockLight
	}
	dst.Sections = sections
	if dst.Heightmaps == nil {
		dst.Heightmaps = make(map[string][]uint64)
	}
	dst.Heightmaps["WORLD_SURFACE_WG"] = c.HeightMaps.WorldSurfaceWG.Raw()
	dst.Heightmaps["WORLD_SURFACE"] = c.HeightMaps.WorldSurface.Raw()
	dst.Heightmaps["OCEAN_FLOOR_WG"] = c.HeightMaps.OceanFloorWG.Raw()
	dst.Heightmaps["OCEAN_FLOOR"] = c.HeightMaps.OceanFloor.Raw()
	dst.Heightmaps["MOTION_BLOCKING"] = c.HeightMaps.MotionBlocking.Raw()
	dst.Heightmaps["MOTION_BLOCKING_NO_LEAVES"] = c.HeightMaps.MotionBlockingNoLeaves.Raw()
	dst.Status = string(c.Status)
	return
}

func writeStatesPalette(paletteData *PaletteContainer[BlocksState]) (palette []save.BlockState, data []uint64, err error) {
	rawPalette := paletteData.palette.export()
	palette = make([]save.BlockState, len(rawPalette))

	var buffer bytes.Buffer
	for i, v := range rawPalette {
		b := block.StateList[v]
		palette[i].Name = b.ID()

		buffer.Reset()
		err = nbt.NewEncoder(&buffer).Encode(b, "")
		if err != nil {
			return
		}
		_, err = nbt.NewDecoder(&buffer).Decode(&palette[i].Properties)
		if err != nil {
			return
		}
	}

	data = make([]uint64, len(paletteData.data.Raw()))
	copy(data, paletteData.data.Raw())
	return
}

func writeBiomesPalette(paletteData *PaletteContainer[BiomesState]) (palette []save.BiomeState, data []uint64, err error) {
	rawPalette := paletteData.palette.export()
	palette = make([]save.BiomeState, len(rawPalette))

	var biomeID []byte
	for i, v := range rawPalette {
		biomeID, err = v.MarshalText()
		if err != nil {
			return
		}
		palette[i] = save.BiomeState(biomeID)
	}

	data = make([]uint64, len(paletteData.data.Raw()))
	copy(data, paletteData.data.Raw())
	return
}

func (c *Chunk) WriteTo(w io.Writer) (int64, error) {
	data, err := c.Data()
	if err != nil {
		return 0, err
	}
	light := lightData{
		SkyLightMask:   make(pk.BitSet, (16*16*16-1)>>6+1),
		BlockLightMask: make(pk.BitSet, (16*16*16-1)>>6+1),
		SkyLight:       []pk.ByteArray{},
		BlockLight:     []pk.ByteArray{},
		AddAdges:       c.AddEdges,
	}
	for i, v := range c.Sections {
		if v.SkyLight != nil {
			light.SkyLightMask.Set(i, true)
			light.SkyLight = append(light.SkyLight, v.SkyLight)
		}
		if v.BlockLight != nil {
			light.BlockLightMask.Set(i, true)
			light.BlockLight = append(light.BlockLight, v.BlockLight)
		}
	}
	return pk.Tuple{
		// Heightmaps
		pk.NBT(struct {
			MotionBlocking []uint64 `nbt:"MOTION_BLOCKING"`
			WorldSurface   []uint64 `nbt:"WORLD_SURFACE"`
		}{
			MotionBlocking: c.HeightMaps.MotionBlocking.Raw(),
			WorldSurface:   c.HeightMaps.WorldSurface.Raw(),
		}),
		pk.ByteArray(data),
		pk.Array(c.BlockEntity),
		&light,
	}.WriteTo(w)
}

func (c *Chunk) ReadFrom(r io.Reader) (int64, error) {
	var (
		heightmaps struct {
			MotionBlocking []uint64 `nbt:"MOTION_BLOCKING"`
			WorldSurface   []uint64 `nbt:"WORLD_SURFACE"`
		}
		data pk.ByteArray
	)

	n, err := pk.Tuple{
		pk.NBT(&heightmaps),
		&data,
		pk.Array(&c.BlockEntity),
		&lightData{
			AddAdges:       c.AddEdges,
			SkyLightMask:   make(pk.BitSet, (16*16*16-1)>>6+1),
			BlockLightMask: make(pk.BitSet, (16*16*16-1)>>6+1),
			SkyLight:       []pk.ByteArray{},
			BlockLight:     []pk.ByteArray{},
		},
	}.ReadFrom(r)
	if err != nil {
		return n, err
	}

	bitsForHeight := bits.Len( /* chunk height in blocks */ uint(len(c.Sections))*16 + 1)
	c.HeightMaps.MotionBlocking = NewBitStorage(bitsForHeight, 16*16, heightmaps.MotionBlocking)
	c.HeightMaps.WorldSurface = NewBitStorage(bitsForHeight, 16*16, heightmaps.WorldSurface)

	err = c.PutData(data)
	return n, err
}

func (c *Chunk) Data() ([]byte, error) {
	var buff bytes.Buffer
	for i := range c.Sections {
		_, err := c.Sections[i].WriteTo(&buff)
		if err != nil {
			return nil, err
		}
	}
	return buff.Bytes(), nil
}

func (c *Chunk) PutData(data []byte) error {
	r := bytes.NewReader(data)
	for i := range c.Sections {
		_, err := c.Sections[i].ReadFrom(r)
		if err != nil {
			return err
		}
	}
	return nil
}

type HeightMaps struct {
	WorldSurfaceWG         *BitStorage // test = NOT_AIR
	WorldSurface           *BitStorage // test = NOT_AIR
	OceanFloorWG           *BitStorage // test = MATERIAL_MOTION_BLOCKING
	OceanFloor             *BitStorage // test = MATERIAL_MOTION_BLOCKING
	MotionBlocking         *BitStorage // test = BlocksMotion or isFluid
	MotionBlockingNoLeaves *BitStorage // test = BlocksMotion or isFluid
}

type BlockEntity struct {
	XZ   int8
	Y    int16
	Type block.EntityType
	Data nbt.RawMessage
}

func (b BlockEntity) UnpackXZ() (X, Z int) {
	return int((uint8(b.XZ) >> 4) & 0xF), int(uint8(b.XZ) & 0xF)
}

func (b *BlockEntity) PackXZ(X, Z int) bool {
	if X > 0xF || Z > 0xF || X < 0 || Z < 0 {
		return false
	}
	b.XZ = int8(X<<4 | Z)
	return true
}

func (b BlockEntity) WriteTo(w io.Writer) (n int64, err error) {
	return pk.Tuple{
		pk.Byte(b.XZ),
		pk.Short(b.Y),
		pk.VarInt(b.Type),
		pk.NBT(b.Data),
	}.WriteTo(w)
}

func (b *BlockEntity) ReadFrom(r io.Reader) (n int64, err error) {
	return pk.Tuple{
		(*pk.Byte)(&b.XZ),
		(*pk.Short)(&b.Y),
		(*pk.VarInt)(&b.Type),
		pk.NBT(&b.Data),
	}.ReadFrom(r)
}

type Section struct {
	BlockCount int16
	States     *PaletteContainer[BlocksState]
	Biomes     *PaletteContainer[BiomesState]
	// Half a byte per light value.
	// Could be nil if not exist
	SkyLight   []byte // len() == 2048
	BlockLight []byte // len() == 2048
}

func (s *Section) GetBlock(i int) BlocksState {
	return s.States.Get(i)
}

func (s *Section) SetBlock(i int, v BlocksState) {
	if !block.IsAir(s.States.Get(i)) {
		s.BlockCount--
	}
	if !block.IsAir(v) {
		s.BlockCount++
	}
	s.States.Set(i, v)
}

func (s *Section) WriteTo(w io.Writer) (int64, error) {
	return pk.Tuple{
		pk.Short(s.BlockCount),
		s.States,
		s.Biomes,
	}.WriteTo(w)
}

func (s *Section) ReadFrom(r io.Reader) (int64, error) {
	return pk.Tuple{
		(*pk.Short)(&s.BlockCount),
		s.States,
		s.Biomes,
	}.ReadFrom(r)
}

type lightData struct {
	AddAdges       bool
	SkyLightMask   pk.BitSet
	BlockLightMask pk.BitSet
	SkyLight       []pk.ByteArray
	BlockLight     []pk.ByteArray
}

func bitSetRev(set pk.BitSet) pk.BitSet {
	rev := make(pk.BitSet, len(set))
	for i := range rev {
		rev[i] = ^set[i]
	}
	return rev
}

func (l *lightData) WriteTo(w io.Writer) (int64, error) {
	return pk.Tuple{
		pk.Opt{
			Has:   func() bool { return l.AddAdges },
			Field: pk.Boolean(true),
		},
		l.SkyLightMask,
		l.BlockLightMask,
		bitSetRev(l.SkyLightMask),
		bitSetRev(l.BlockLightMask),
		pk.Array(l.SkyLight),
		pk.Array(l.BlockLight),
	}.WriteTo(w)
}

func (l *lightData) ReadFrom(r io.Reader) (int64, error) {
	var TrustEdges pk.Boolean
	var RevSkyLightMask, RevBlockLightMask pk.BitSet
	return pk.Tuple{
		pk.Opt{
			Has:   func() bool { return l.AddAdges },
			Field: &TrustEdges,
		},
		&l.SkyLightMask,
		&l.BlockLightMask,
		&RevSkyLightMask,
		&RevBlockLightMask,
		pk.Array(&l.SkyLight),
		pk.Array(&l.BlockLight),
	}.ReadFrom(r)
}
