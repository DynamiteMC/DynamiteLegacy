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

var playerCountText *widget.RichText
var playerContainer *widget.List

func LaunchGUI() fyne.Window {
	app := app.New()
	window := app.NewWindow("Dynamite Server")
	title := widget.NewRichTextFromMarkdown("# Dynamite Server")
	consoleTitle := widget.NewRichTextFromMarkdown("## Console")
	server.Logger.GUIConsole = widget.NewTextGridFromString(strings.Join(server.Logger.ConsoleText, "\n"))
	command := widget.NewEntry()
	command.SetPlaceHolder("Input a command")
	command.OnSubmitted = func(s string) {
		server.Command("console", s)
		command.SetText("")
	}
	console := container.NewBorder(consoleTitle, command, nil, nil, container.NewScroll(server.Logger.GUIConsole))

	playersTitle := widget.NewRichTextFromMarkdown("## Players")
	max := fmt.Sprint(server.Config.MaxPlayers)
	if max == "-1" {
		max = "Unlimited"
	}
	playerCountText = widget.NewRichTextFromMarkdown(fmt.Sprintf("### %d/%s players", len(server.Players.Players), max))
	playerContainer = widget.NewList(
		func() int {
			return len(server.Players.Players)
		},
		func() fyne.CanvasObject {
			return container.NewHBox()
		},
		func(i widget.ListItemID, o fyne.CanvasObject) {
			cont := o.(*fyne.Container)
			player := server.Players.Players[server.Players.PlayerIDs[i]]
			if len(cont.Objects) == 0 {
				res, _ := http.Get(fmt.Sprintf("https://crafatar.com/avatars/%s", player.UUID.String))
				skinData, _ := io.ReadAll(res.Body)
				skin := widget.NewIcon(fyne.NewStaticResource("skin", skinData))
				skin.Resize(fyne.NewSize(640, 640))
				cont.Objects = append(cont.Objects, skin, widget.NewRichTextFromMarkdown("### "+player.Name))
				cont.Refresh()
			}
		})
	/*for _, player := range server.Players.Players {
		res, _ := http.Get(fmt.Sprintf("https://crafatar.com/avatars/%s", player.UUID))
		skinData, _ := io.ReadAll(res.Body)
		skin := widget.NewIcon(fyne.NewStaticResource("skin", skinData))
		skin.Resize(fyne.NewSize(640, 640))
		cont := container.NewHBox(skin, widget.NewRichTextFromMarkdown("### "+player.Name))
		playerContainer.Add(cont)
	}*/
	players := container.NewBorder(container.NewVBox(playersTitle, playerCountText), nil, nil, nil, playerContainer)
	sp := container.NewHSplit(console, players)
	sp.SetOffset(0.6)
	window.SetContent(container.NewBorder(title, nil, nil, nil, sp))
	window.Resize(fyne.NewSize(700, 300))
	return window
}
