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

type Messages struct {
	NotInWhitelist string `yaml:"not_in_whitelist"`
	Banned         string `yaml:"banned"`
}

type Icon struct {
	Path   string `yaml:"path"`
	Enable bool   `yaml:"enable"`
}

type Config struct {
	TCP        TCP      `yaml:"java"`
	UDP        UDP      `yaml:"bedrock"`
	MOTD       string   `yaml:"motd"`
	Icon       Icon     `yaml:"icon"`
	Whitelist  bool     `yaml:"whitelist"`
	Gamemode   string   `yaml:"gamemode"`
	MaxPlayers int      `yaml:"max_players"`
	Online     bool     `yaml:"online_mode"`
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
			MaxPlayers: 100,
			Online:     true,
			Messages: Messages{
				NotInWhitelist: "You are not whitelisted.",
				Banned:         "You are banned from this server.",
			},
			Icon: Icon{
				Path:   "server-icon.png",
				Enable: false,
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
