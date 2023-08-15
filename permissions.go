package main

import (
	"encoding/json"
	"fmt"
	"os"
)

/*
	Permissions:
		server.stop - /stop command
		server.reload - /reload command
		server.chat - Use chat
		server.chat.colors - Use chat colors
*/

var groupCache = make(map[string]GroupPermissions)
var playerCache = make(map[string]PlayerPermissions)

type PlayerPermissions struct {
	Group       string          `json:"group"`
	Permissions map[string]bool `json:"permissions"`
}

type GroupPermissions struct {
	DisplayName string          `json:"display_name"`
	Prefix      string          `json:"prefix"`
	Suffix      string          `json:"suffix"`
	Permissions map[string]bool `json:"permissions"`
}

func getPlayer(playerId string) PlayerPermissions {
	if playerCache[playerId].Permissions != nil {
		return playerCache[playerId]
	}
	d, err := os.ReadFile(fmt.Sprintf("permissions/players/%s.json", playerId))
	if err != nil {
		os.WriteFile(fmt.Sprintf("permissions/players/%s.json", playerId), []byte(`{"group":"default"}`), 0755)
		return PlayerPermissions{}
	}
	var data PlayerPermissions
	json.Unmarshal(d, &data)
	return data
}

func getGroup(group string) GroupPermissions {
	if groupCache[group].Permissions != nil {
		return groupCache[group]
	}
	d, err := os.ReadFile(fmt.Sprintf("permissions/groups/%s.json", group))
	if err != nil {
		return GroupPermissions{}
	}
	var data GroupPermissions
	json.Unmarshal(d, &data)
	return data
}

func (server Server) HasPermissions(playerId string, perms []string) bool {
	if playerId == "console" {
		return true
	}
	if len(perms) == 0 {
		return true
	}
	for i := 0; i < len(server.OPs); i++ {
		if server.OPs[i].UUID == playerId {
			return true
		}
	}
	permissionsPlayer := getPlayer(playerId)
	permissionsGroup := getGroup(permissionsPlayer.Group)
	for _, perm := range perms {
		if !permissionsPlayer.Permissions[perm] && !permissionsGroup.Permissions[perm] {
			return false
		}
	}
	return true
}
