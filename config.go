package main

import (
	"os"

	"gopkg.in/yaml.v2"
)

type TCP struct {
	ServerIP   string `yaml:"server_ip"`
	ServerPort int    `yaml:"server_port"`
	Enable     bool   `yaml:"enable"`
}

type UDP struct {
	ServerIP   string `yaml:"server_ip"`
	ServerPort int    `yaml:"server_port"`
	Enable     bool   `yaml:"enable"`
}

type Tablist struct {
	Header []string `yaml:"header"`
	Footer []string `yaml:"footer"`
}

type Messages struct {
	NotInWhitelist string `yaml:"not_in_whitelist"`
	Banned         string `yaml:"banned"`
	ServerFull     string `yaml:"server_full"`
	AlreadyPlaying string `yaml:"already_playing"`
	PlayerJoin     string `yaml:"player_join"`
	PlayerLeave    string `yaml:"player_leave"`
	UnknownCommand string `yaml:"unknown_command"`
}

type Icon struct {
	Path   string `yaml:"path"`
	Enable bool   `yaml:"enable"`
}

type Chat struct {
	Format string `yaml:"format"`
	Colors bool   `yaml:"colors"`
	Enable bool   `yaml:"enable"`
}

type Config struct {
	TCP        TCP      `yaml:"java"`
	UDP        UDP      `yaml:"bedrock"`
	MOTD       string   `yaml:"motd"`
	Icon       Icon     `yaml:"icon"`
	Whitelist  bool     `yaml:"whitelist"`
	Gamemode   string   `yaml:"gamemode"`
	Hardcore   bool     `yaml:"hardcore"`
	MaxPlayers int      `yaml:"max_players"`
	Online     bool     `yaml:"online_mode"`
	Tablist    Tablist  `yaml:"tablist"`
	Chat       Chat     `yaml:"chat"`
	Messages   Messages `yaml:"messages"`
}

func LoadConfig() *Config {
	config := &Config{}

	file, err := os.Open("gocraft.yml")
	if err != nil {
		file.Close()
		config = &Config{
			TCP: TCP{
				ServerIP:   "0.0.0.0",
				ServerPort: 25565,
				Enable:     true,
			},
			UDP: UDP{
				ServerIP:   "0.0.0.0",
				ServerPort: 19132,
				Enable:     false,
			},
			MOTD:       "A GoCraft Minecraft Server",
			Whitelist:  false,
			Gamemode:   "survival",
			Hardcore:   false,
			MaxPlayers: 100,
			Online:     true,
			Messages: Messages{
				NotInWhitelist: "You are not whitelisted.",
				Banned:         "You are banned from this server.",
				ServerFull:     "The server is full.",
				AlreadyPlaying: "You are already playing on this server with a different client.",
				PlayerJoin:     "§e%player% has joined the game",
				PlayerLeave:    "§e%player% has left the game",
				UnknownCommand: "§cUnknown or incomplete command",
			},
			Icon: Icon{
				Path:   "server-icon.png",
				Enable: false,
			},
			Tablist: Tablist{
				Header: []string{},
				Footer: []string{},
			},
			Chat: Chat{
				Colors: false,
				Format: "<%player%> %message%",
				Enable: true,
			},
		}
		file, _ := os.Create("gocraft.yml")
		e := yaml.NewEncoder(file)
		e.Encode(&config)
		return config
	}
	defer file.Close()

	d := yaml.NewDecoder(file)

	if err := d.Decode(&config); err != nil {
		return nil
	}

	return config
}
