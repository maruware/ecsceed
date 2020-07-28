package ecsceed

import (
	"encoding/json"
	"fmt"
	"log"
)

func (a *App) Log(v ...interface{}) {
	args := []interface{}{a.Name()}
	args = append(args, v...)
	log.Println(args...)
}

func (a *App) DebugLog(v ...interface{}) {
	if !a.Debug {
		return
	}
	a.Log(v...)
}

func (a *App) LogJSON(v interface{}) {
	b, _ := json.Marshal(v)
	fmt.Print(b)
}
