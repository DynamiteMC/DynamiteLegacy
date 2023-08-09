package main

import (
	"encoding/json"
)

type Version struct {
	Name     string `json:"name"`
	Protocol int    `json:"protocol"`
}

type Players struct {
	Max    int      `json:"max"`
	Online int      `json:"online"`
	Sample []Player `json:"sample"`
}

type Description struct {
	Text string `json:"text"`
}

type StatusResponse struct {
	Version            Version     `json:"version"`
	Players            Players     `json:"players"`
	Description        Description `json:"description"`
	EnforcesSecureChat bool        `json:"enforcesSecureChat"`
	PreviewsChat       bool        `json:"previewsChat"`
	Favicon            string      `json:"favicon"`
}

func CreateStatusResponse(data StatusResponse) string {
	buffer, err := json.Marshal(&data)
	if err != nil {
		server.Logger.Error("Failed to create StatusResponse packet")
	}
	return string(buffer)
}
