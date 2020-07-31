package ecsceed

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/fatih/color"
)

func (a *App) Log(v ...interface{}) {
	log.Println(v...)
}

func (a *App) DebugLog(v ...interface{}) {
	if !a.Debug {
		return
	}

	debug := color.MagentaString("[DEBUG]")
	args := []interface{}{debug}
	args = append(args, v...)

	a.Log(args...)
}

func LogDone() string {
	return color.GreenString("\u2713")
}

func LogTarget(v interface{}) string {
	return color.CyanString("%+v", v)
}

func (a *App) LogJSON(v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Print(string(b) + "\n")
}
