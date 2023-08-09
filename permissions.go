package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type PlayerPermissions struct {
	Group       string          `json:"group"`
	Permissions map[string]bool `json:"permissions"`
}

func getPermissions(playerId string) PlayerPermissions {
	d, err := os.ReadFile(fmt.Sprintf("permissions/players/%s.json", playerId))
	if err != nil {
		os.WriteFile(fmt.Sprintf("permissions/players/%s.json", playerId), []byte("{}"), 0755)
		return PlayerPermissions{}
	}
	var data PlayerPermissions
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
	ops := LoadPlayerList("whitelist.json")
	for i := 0; i < len(ops); i++ {
		if ops[i].UUID == playerId {
			return true
		}
	}
	permissions := getPermissions(playerId)
	for _, perm := range perms {
		if !permissions.Permissions[perm] {
			return false
		}
	}
	return true
}
