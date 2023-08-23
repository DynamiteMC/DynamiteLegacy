package main

import (
	"errors"
	"gocraft/logger"
	"sync"

	"github.com/Tnze/go-mc/level"
	"github.com/Tnze/go-mc/net"
	"github.com/Tnze/go-mc/net/packet"

	"github.com/Tnze/go-mc/save"
	"github.com/Tnze/go-mc/yggdrasil/user"
)

var ErrChunkNotExist = errors.New("ErrChunkNotExist")

type Playerlist struct{}

type World struct {
	TickLock *sync.Mutex
	Name     string
	Chunks   map[[2]int32]*LoadedChunk
}

type Enchantment struct {
	ID    string `nbt:"id"`
	Level int32  `nbt:"lvl"`
}

type InventorySlotTag struct {
	Damage       int32         `nbt:"Damage"`
	RepairCost   int32         `nbt:"RepairCost"`
	Enchantments []Enchantment `nbt:"Enchantments"`
}

type InventorySlot struct {
	Count int              `nbt:"Count"`
	Slot  int              `nbt:"Slot"`
	ID    string           `nbt:"id"`
	Tag   InventorySlotTag `nbt:"tag"`
}

type Inventory []InventorySlot

type PlayerDataRecipeBook struct {
	IsBlastingFurnaceFilteringCraftable int32         `nbt:"isBlastingFurnaceFilteringCraftable"`
	IsBlastingFurnaceGuiOpen            int32         `nbt:"isBlastingFurnaceGuiOpen"`
	IsFilteringCraftable                int32         `nbt:"isFilteringCraftable"`
	IsFurnaceFilteringCraftable         int32         `nbt:"isFurnaceFilteringCraftable"`
	IsFurnaceGuiOpen                    int32         `nbt:"isFurnaceGuiOpen"`
	IsGuiOpen                           int32         `nbt:"isGuiOpen"`
	IsSmokerFilteringCraftables         int32         `nbt:"isSmokerFilteringCraftables"`
	IsSmokerGuiOpen                     int32         `nbt:"isSmokerGuiOpen"`
	Recipes                             []interface{} `nbt:"recipes"`
	ToBeDisplayed                       []interface{} `nbt:"toBeDisplayed"`
}

type PlayerData struct {
	Attributes          []interface{}        `nbt:"Attributes"`
	OnGround            int32                `nbt:"OnGround"`
	Health              float32              `nbt:"Health"`
	Dimension           string               `nbt:"Dimension"`
	Fire                int32                `nbt:"Fire"`
	Score               int32                `nbt:"Score"`
	SelectedItemSlot    int32                `nbt:"SelectedItemSlot"`
	EnderItems          []interface{}        `nbt:"EnderItems"`
	Inventory           Inventory            `nbt:"Inventory"`
	Pos                 []float64            `nbt:"Pos"`
	Motion              []interface{}        `nbt:"Motion"`
	Rotation            []float32            `nbt:"Rotation"`
	XpLevel             int32                `nbt:"XpLevel"`
	XpTotal             int32                `nbt:"XpTotal"`
	XpP                 float32              `nbt:"XpP"`
	DeathTime           int32                `nbt:"DeathTime"`
	HurtTime            int32                `nbt:"HurtTime"`
	SleepTimer          int32                `nbt:"SleepTimer"`
	SeenCredits         int32                `nbt:"seenCredits"`
	PlayerGameType      int32                `nbt:"playerGameType"`
	FoodLevel           int32                `nbt:"foodLevel"`
	FoodExhaustionLevel float32              `nbt:"foodExhaustionLevel"`
	FoodSaturationLevel float32              `nbt:"foodSaturationLevel"`
	FoodTickTimer       int32                `nbt:"foodTickTimer"`
	RecipeBook          PlayerDataRecipeBook `nbt:"recipeBook"`
}

type Player struct {
	sync.Mutex
	Name         string
	UUID         UUID
	Connection   *net.Conn
	Properties   []user.Property
	Client       ClientData
	IP           string
	Position     [3]int32
	OldPosition  [3]int32
	Rotation     [2]float32
	ChunkPos     [3]int32
	LoadedChunks map[[2]int32]struct{}
	LoadQueue    [][2]int32
	UnloadQueue  [][2]int32
	Data         PlayerData
	LastTick     uint
	EntityID     int
}

const (
	CHAT_ENABLED = iota
	CHAT_COMMANDS_ONLY
	CHAT_HIDDEN
)

const (
	LEFT_HAND = iota
	RIGHT_HAND
)

const (
	FAVICON_NOTFOUND = iota
	FAVICON_INVALID_FORMAT
	FAVICON_INVALID_DIMENSIONS
)

const (
	PLUGINCODE_INFO = iota
	PLUGINCODE_LOG
)

type Events struct {
	_Events map[string][]func(...interface{})
}

type ClientData struct {
	Locale              packet.String
	ViewDistance        packet.Byte
	ChatMode            packet.VarInt
	ChatColors          packet.Boolean
	DisplayedSkinParts  packet.UnsignedByte
	MainHand            packet.VarInt
	EnableTextFiltering packet.Boolean
	AllowServerListings packet.Boolean
	Brand               packet.String
}

type PlayersC struct {
	*sync.Mutex
	Players       map[string]*Player
	PlayerNames   map[string]string
	PlayerIDs     []string
	Whitelist     []PlayerBase
	OPs           []PlayerBase
	BannedPlayers []PlayerBase
	BannedIPs     []string
}

type Server struct {
	Commands        map[string]Command
	Players         PlayersC
	Events          Events
	Config          *Config
	Logger          *logger.Logger
	Playerlist      Playerlist
	StartTime       int64
	Favicon         []byte
	Level           save.Level
	Listener        *net.Listener
	EntityCounter   int
	TeleportCounter int
	Mojang          MojangAPI
	Worlds          map[string]World
}

type Node struct {
	Parent     int
	Children   []int
	Data       interface{}
	EntryIndex int
}

type CommandGraph struct {
	PlayerID string
}

type Parser struct {
	ID         int
	Name       string
	Properties packet.FieldEncoder
}

type Argument struct {
	Name                string
	Redirect            int
	RequiredPermissions []string
	SuggestionsType     string
	Parser              Parser
	Optional            bool
}

type Command struct {
	Name                string
	Redirect            int
	RequiredPermissions []string
	Arguments           []Argument
	Aliases             []string
}

type UUID struct {
	String string
	Binary packet.UUID
}

type LoadedChunk struct {
	sync.Mutex
	Viewers []string
	*level.Chunk
}
