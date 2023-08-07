package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
)

func HasArg(arg string) bool {
	for _, s := range os.Args {
		if s == arg {
			return true
		}
	}
	return false
}

type Logger struct {
	FilePath string
}

func (logger Logger) Append(str string) {
	consoleText = append(consoleText, str)
	if guiConsole != nil {
		guiConsole.SetText(strings.Join(consoleText, "\n"))
	}
}

func getDateString() string {
	return time.Now().Format("15:04:05")
}

func (logger Logger) Info(data ...interface{}) {
	blue := color.New(color.BgBlue).Add(color.FgWhite).Add(color.Bold).SprintFunc()
	str := ""
	for _, d := range data {
		str += fmt.Sprintf("%v ", d)
	}
	time := getDateString()
	logger.Append(fmt.Sprintf("%s [INFO] %s", time, str))
	fmt.Println(time, blue("INFO"), str)
}

func (logger Logger) Print(data ...interface{}) {
	str := ""
	for _, d := range data {
		str += fmt.Sprintf("%v ", d)
	}
	logger.Append(str)
	fmt.Println(str)
}

func (logger Logger) Debug(data ...interface{}) {
	if !HasArg("-debug") {
		return
	}
	cyan := color.New(color.BgCyan).Add(color.FgWhite).Add(color.Bold).SprintFunc()
	str := ""
	for _, d := range data {
		str += fmt.Sprintf("%v ", d)
	}
	time := getDateString()
	logger.Append(fmt.Sprintf("%s [DEBUG] %s", time, str))
	fmt.Println(time, cyan("DEBUG"), str)
}

func (logger Logger) Error(data ...interface{}) {
	red := color.New(color.BgRed).Add(color.FgWhite).Add(color.Bold).SprintFunc()
	str := ""
	for _, d := range data {
		str += fmt.Sprintf("%v ", d)
	}
	time := getDateString()
	logger.Append(fmt.Sprintf("%s [ERROR] %s", time, str))
	fmt.Println(time, red("ERROR"), str)
}

func (logger Logger) Warn(data ...interface{}) {
	yellow := color.New(color.BgYellow).Add(color.FgWhite).Add(color.Bold).SprintFunc()
	str := ""
	for _, d := range data {
		str += fmt.Sprintf("%v ", d)
	}
	time := getDateString()
	logger.Append(fmt.Sprintf("%s [WARN] %s", time, str))
	fmt.Println(time, yellow("WARN"), str)
}
