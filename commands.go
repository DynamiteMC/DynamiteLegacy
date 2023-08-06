package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func ReloadConfig() {
	config = LoadConfig()
	fmt.Println("Reloaded config successfully")
}

func CreateSTDINReader() {
	reader := bufio.NewReader(os.Stdin)
	command, _ := reader.ReadString('\n')
	Command(command)
	go CreateSTDINReader()
}

func Command(command string) {
	command = strings.TrimSpace(command)
	switch command {
	case "reload-config":
		{
			ReloadConfig()
		}
	default:
		{
			fmt.Println("Unknown command. Please run 'help' for a list of commands.")
		}
	}
}
