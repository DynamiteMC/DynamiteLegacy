package main

import (
	"bufio"
	"os"
	"strings"

	"github.com/Tnze/go-mc/chat"
)

func ReloadConfig() chat.Message {
	server.Config = LoadConfig()
	return chat.Text("Reloaded config successfully")
}

func CreateSTDINReader() {
	reader := bufio.NewReader(os.Stdin)
	command, _ := reader.ReadString('\n')
	logger.Print(Command(command))
	go CreateSTDINReader()
}

var Commands = []string{"stop", "reload"}

func Command(command string) chat.Message {
	command = strings.TrimSpace(command)
	switch command {
	case "reload":
		{
			return ReloadConfig()
		}
	case "stop":
		{
			go func() {
				os.Exit(0)
			}()
			return chat.Text("Shutting down server...")
		}
	default:
		{
			return chat.Text(server.Config.Messages.UnknownCommand)
		}
	}
}
