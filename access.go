package main

import (
	"encoding/json"
	"os"
)

func LoadPlayerList(path string) []Player {
	list := []Player{}

	file, err := os.Open(path)
	if err != nil {
		file.Close()
		file, _ := os.Create(path)
		e := json.NewEncoder(file)
		e.Encode(&list)
		return list
	}
	defer file.Close()

	d := json.NewDecoder(file)

	if err := d.Decode(&list); err != nil {
		return nil
	}

	return list
}

func LoadIPBans() []string {
	list := []string{}

	file, err := os.Open("banned_ips.json")
	if err != nil {
		file.Close()
		file, _ := os.Create("banned_ips.json")
		e := json.NewEncoder(file)
		e.Encode(&list)
		return list
	}
	defer file.Close()

	d := json.NewDecoder(file)

	if err := d.Decode(&list); err != nil {
		return nil
	}

	return list
}

/*

0: User is valid
1: User is not in whitelist
2: User is banned

*/

func ValidatePlayer(name string, id string, ip string) int {
	whitelist := LoadPlayerList("whitelist.json")
	bannedPlayers := LoadPlayerList("banned_players.json")
	bannedIPs := LoadIPBans()
	if config.Whitelist {
		for _, player := range whitelist {
			if player.UUID == id {
				return 0
			}
		}
		return 1
	}
	for _, player := range bannedPlayers {
		if player.UUID == id {
			return 2
		}
	}
	for _, ip := range bannedIPs {
		if ip == ip {
			return 2
		}
	}
	return 0
}