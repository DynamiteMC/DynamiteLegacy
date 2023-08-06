package main

import (
	"fmt"
	"os"

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

func (logger Logger) Info(data ...interface{}) {
	blue := color.New(color.BgBlue).Add(color.FgWhite).Add(color.Bold).SprintFunc()
	str := ""
	for _, d := range data {
		str += fmt.Sprintf("%v ", d)
	}
	fmt.Println(blue("INFO"), str)
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
	fmt.Println(cyan("DEBUG"), str)
}

func (logger Logger) Error(data ...interface{}) {
	red := color.New(color.BgRed).Add(color.FgWhite).Add(color.Bold).SprintFunc()
	str := ""
	for _, d := range data {
		str += fmt.Sprintf("%v ", d)
	}
	fmt.Printf("%s %s\n", red("ERROR"), str)
}
