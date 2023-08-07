package main

import (
	"bufio"
	"os"
	"strings"
)

func ReloadConfig() {
	server.Config = LoadConfig()
	logger.Print("Reloaded config successfully")
}

func CreateSTDINReader() {
	reader := bufio.NewReader(os.Stdin)
	command, _ := reader.ReadString('\n')
	Command(command)
	go CreateSTDINReader()
}

var Commands = []string{"stop", "reload"}

func Command(command string) {
	command = strings.TrimSpace(command)
	switch command {
	case "reload":
		{
			ReloadConfig()
		}
	case "stop":
		{
			logger.Info("Shutting down server...")
			os.Exit(0)
		}
	default:
		{
			logger.Print("Unknown command. Please run 'help' for a list of commands.")
		}
	}
}
