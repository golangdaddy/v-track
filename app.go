package main

import (
    "sync"

    "encoding/json"

	"tinygo.org/x/bluetooth"
)

type App struct {
    adapter *bluetooth.Adapter
    discovered chan bluetooth.ScanResult
    db map[string][]string
    seen map[string]*StoredDevice
    notSeen map[string]*StoredDevice
    sync.RWMutex
}

func (app *App) Marshal(src interface{}) ([]byte, error) {
    return json.Marshal(src)
}

func (app *App) Unmarshal(b []byte, dst interface{}) error {
    return json.Unmarshal(b, dst)
}

func (app *App) see(uid string, sd *StoredDevice) {
    app.Lock()
    defer app.Unlock()
    delete(app.notSeen, uid)
    app.seen[uid] = sd
}

func (app *App) unsee(uid string, sd *StoredDevice) {
    app.Lock()
    defer app.Unlock()
    delete(app.seen, uid)
    app.notSeen[uid] = sd
}
