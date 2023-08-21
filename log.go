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

func (logger Logger) Info(format string, a ...interface{}) {
	blue := color.New(color.FgBlue).Add(color.Bold).SprintFunc()
	time := getDateString()
	str := fmt.Sprintf(format, a...)
	logger.Append(fmt.Sprintf("[%s INFO]: %s", time, str))
	fmt.Printf("[%s %s]: %s\n", time, blue("INFO"), str)
}

func (logger Logger) Print(format string, a ...interface{}) {
	format += "\n"
	logger.Append(format)
	fmt.Printf(format, a...)
}

func (logger Logger) Debug(format string, a ...interface{}) {
	if !HasArg("-debug") {
		return
	}
	cyan := color.New(color.FgCyan).Add(color.Bold).SprintFunc()
	str := fmt.Sprintf(format, a...)
	time := getDateString()
	logger.Append(fmt.Sprintf("[%s DEBUG]: %s", time, str))
	fmt.Printf("[%s %s]: %s\n", time, cyan("DEBUG"), str)
}

func (logger Logger) Error(format string, a ...interface{}) {
	red := color.New(color.FgRed).Add(color.Bold).SprintFunc()

	time := getDateString()
	str := fmt.Sprintf(format, a...)
	logger.Append(fmt.Sprintf("[%s ERROR]: %s", time, str))
	fmt.Printf("[%s %s]: %s\n", time, red("ERROR"), str)
}

func (logger Logger) Warn(format string, a ...interface{}) {
	yellow := color.New(color.FgYellow).Add(color.Bold).SprintFunc()

	time := getDateString()
	str := fmt.Sprintf(format, a...)
	logger.Append(fmt.Sprintf("[%s WARN]: %s", time, str))
	fmt.Printf("[%s %s]: %s\n", time, yellow("WARN"), str)
}
