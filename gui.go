package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

var consoleText []string
var guiConsole *widget.TextGrid
var playerCountText *widget.RichText
var playerContainer *fyne.Container

func LaunchGUI() fyne.Window {
	app := app.New()
	window := app.NewWindow("GoCraft Server")
	title := widget.NewRichTextFromMarkdown("# GoCraft Server")

	consoleTitle := widget.NewRichTextFromMarkdown("## Console")
	guiConsole = widget.NewTextGridFromString(strings.Join(consoleText, "\n"))
	command := widget.NewEntry()
	command.SetPlaceHolder("Input a command")
	command.OnSubmitted = func(s string) {
		Command(s)
		command.SetText("")
	}
	console := container.NewBorder(consoleTitle, command, nil, nil, container.NewScroll(guiConsole))

	playersTitle := widget.NewRichTextFromMarkdown("## Players")
	max := fmt.Sprint(server.Config.MaxPlayers)
	if max == "-1" {
		max = "Unlimited"
	}
	playerCountText = widget.NewRichTextFromMarkdown(fmt.Sprintf("### %d/%s players", len(server.Players), max))
	playerContainer = container.NewVBox()
	for _, player := range server.Players {
		res, _ := http.Get(fmt.Sprintf("https://crafatar.com/avatars/%s", player.UUID))
		skinData, _ := io.ReadAll(res.Body)
		skin := widget.NewIcon(fyne.NewStaticResource("skin", skinData))
		skin.Resize(fyne.NewSize(640, 640))
		cont := container.NewHBox(skin, widget.NewRichTextFromMarkdown("### "+player.Name))
		playerContainer.Add(cont)
	}
	players := container.NewBorder(container.NewVBox(playersTitle, playerCountText), nil, nil, nil, playerContainer)

	sp := container.NewHSplit(console, players)
	sp.SetOffset(0.6)
	window.SetContent(container.NewBorder(title, nil, nil, nil, sp))
	window.Resize(fyne.NewSize(700, 300))
	return window
}
