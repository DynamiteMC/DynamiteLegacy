package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func getPermissions(playerId string) map[string]bool {
	d, err := os.ReadFile(fmt.Sprintf("permissions/%s.json", playerId))
	if err != nil {
		os.WriteFile(fmt.Sprintf("permissions/%s.json", playerId), []byte("{}"), 0755)
		return make(map[string]bool)
	}
	var data map[string]bool
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
		if !permissions[perm] {
			return false
		}
	}
	return true
}
