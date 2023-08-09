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

func WritePlayerList(path string, player Player) {
	list := []Player{}

	file, err := os.Open(path)
	if err != nil {
		file.Close()
		file, _ := os.Create(path)
		e := json.NewEncoder(file)
		e.Encode(&list)
	}
	defer file.Close()
	var data []Player
	json.NewDecoder(file).Decode(&data)
	data = append(data, player)
	json.NewEncoder(file).Encode(&data)
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
3: Server is full
4: User is already playing on another client

*/

func ValidatePlayer(name string, id string, ip string) int {
	whitelist := LoadPlayerList("whitelist.json")
	bannedPlayers := LoadPlayerList("banned_players.json")
	bannedIPs := LoadIPBans()
	for _, player := range bannedPlayers {
		if player.UUID == id {
			return 2
		}
	}
	for _, i := range bannedIPs {
		if i == ip {
			return 2
		}
	}
	if server.Config.Whitelist.Enable {
		d := false
		for _, player := range whitelist {
			if player.UUID == id {
				d = true
				break
			}
		}
		if !d {
			return 1
		}
	}
	if server.Players[id].UUID == id {
		return 4
	}
	if server.Config.MaxPlayers == -1 {
		return 0
	}
	if len(server.Players) >= server.Config.MaxPlayers {
		return 3
	}
	return 0
}
