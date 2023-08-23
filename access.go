package main

import (
	"encoding/json"
	"os"
)

type PlayerBase struct {
	UUID string `json:"id"`
	Name string `json:"name"`
}

func LoadPlayerList(path string) []PlayerBase {
	list := []PlayerBase{}

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

func WritePlayerList(path string, player PlayerBase) []PlayerBase {
	list := []PlayerBase{}

	b, err := os.ReadFile(path)
	if err != nil {
		list = append(list, player)
		data, _ := json.Marshal(list)
		os.WriteFile(path, data, 0755)
	}
	json.Unmarshal(b, &list)
	list = append(list, player)
	data, _ := json.Marshal(list)
	os.WriteFile(path, data, 0755)
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
3: Server is full
4: User is already playing on another client

*/

func ValidatePlayer(name string, id string, ip string) int {
	for _, player := range server.Players.BannedPlayers {
		if player.UUID == id {
			return 2
		}
	}
	for _, i := range server.Players.BannedIPs {
		if i == ip {
			return 2
		}
	}
	if server.Config.Whitelist.Enable {
		d := false
		for _, player := range server.Players.Whitelist {
			if player.UUID == id {
				d = true
				break
			}
		}
		if !d {
			return 1
		}
	}
	if server.Players.Players[id] != nil {
		return 4
	}
	if server.Config.MaxPlayers == -1 {
		return 0
	}
	if len(server.Players.Players) >= server.Config.MaxPlayers {
		return 3
	}
	return 0
}
