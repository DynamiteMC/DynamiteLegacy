package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type MojangAPI struct{}

func (api MojangAPI) ParseUUID(uuid string) string {
	str := ""
	for i, char := range strings.Split(uuid, "") {
		str += char
		if i == 7 || i == 11 || i == 15 || i == 19 {
			str += "-"
		}
	}
	return str
}

func (api MojangAPI) FetchUsername(username string) (bool, Player) {
	resp, err := http.Get(fmt.Sprintf("https://api.mojang.com/users/profiles/minecraft/%s", username))
	var player Player
	if err != nil {
		return false, player
	}
	body, _ := io.ReadAll(resp.Body)
	var data map[string]string
	err = json.Unmarshal(body, &data)
	if err != nil {
		return false, player
	}
	if data["errorMessage"] != "" {
		return false, player
	}
	player.UUID = api.ParseUUID(data["id"])
	player.Name = data["name"]
	return true, player
}

func (api MojangAPI) FetchUUID(uuid string) (bool, Player) {
	resp, err := http.Get(fmt.Sprintf("https://api.mojang.com/users/profiles/minecraft/%s", uuid))
	var player Player
	if err != nil {
		return false, player
	}
	body, _ := io.ReadAll(resp.Body)
	var data map[string]string
	err = json.Unmarshal(body, &data)
	if err != nil {
		return false, player
	}
	if data["errorMessage"] != "" {
		return false, player
	}
	player.UUID = api.ParseUUID(data["id"])
	player.Name = data["name"]
	return true, player
}
