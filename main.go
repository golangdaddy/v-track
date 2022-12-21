package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"io/fs"
	"path/filepath"

	"github.com/fatih/color"
	"tinygo.org/x/bluetooth"
)

type StoredDevice struct {
	UID        string
	LastSeen   time.Time
	LocalName  string
	CustomName string
	Seen       int
}

type DeviceTuple struct {
	K string
	V *StoredDevice
}

func (app *App) checkSeen() {
	for {
		time.Sleep(5 * time.Second)
		app.RLock()
		list := []*DeviceTuple{}
		for k, sd := range app.seen {
			if sd.Seen != 0 {
				color.Green("SEEN %s %v", k, sd)
			} else {
				color.Yellow("SEEN %s %v", k, sd)
			}
			list = append(
				list,
				&DeviceTuple{
					K: k,
					V: sd,
				},
			)
		}
		sort.Slice(
			list,
			func(j, k int) bool {
				return list[k].V.LastSeen.After(list[j].V.LastSeen)
			},
		)

		color.Red("%d Total AWOL", len(app.notSeen))
		for k, sd := range app.notSeen {
			if sd.Seen > 1 && (time.Since(sd.LastSeen) > (3 * time.Minute)) {
				color.Red("NOT SEEN %s %v", k, sd)
			}
		}
		app.RUnlock()
	}
}

func (app *App) loadManufacturers() {
	b, err := os.ReadFile("./manufacturers.csv")
	if err != nil {
		panic(err)
	}

	var p int
	for _, bb := range bytes.Split(b, []byte("\n")) {

		x := bytes.Split(bb, []byte("\t"))
		if len(x) < 2 {
			continue
		}

		addr := string(x[0])
		name := string(x[1])
		var descr string
		if len(x) > 2 {
			descr = string(x[2])
		}
		addrParts := strings.Split(addr, ":")
		if len(addrParts) > 3 {
			addr = strings.Join(addrParts[:3], ":")
		}

		entry := fmt.Sprintf("%s: %s", name, descr)

		println(p, addr, entry)

		app.db[addr] = append(
			app.db[addr],
			entry,
		)

		p++
	}
}

func (app *App) loadSeen(m map[string]struct{}) {
	if err := filepath.WalkDir(
		"./seen",
		func(path string, info fs.DirEntry, err error) error {
			println(path)
			if path == "./seen" {
				return nil
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sd := &StoredDevice{}
			if err := app.Unmarshal(b, sd); err != nil {
				return err
			}
			uid := strings.Split(path, "/")[1]
			if _, ok := m[uid]; ok {
				return nil
			}

			if time.Since(sd.LastSeen) > (2 * time.Minute) {
				app.unsee(uid, sd)
			} else {
				app.see(uid, sd)
			}
			return nil
		},
	); err != nil {
		panic(err)
	}
}

func main() {

	os.Mkdir("seen", 0777)
	os.Mkdir("skipped", 0777)

	app := &App{
		adapter:    bluetooth.DefaultAdapter,
		discovered: make(chan bluetooth.ScanResult, 10),
		db:         map[string][]string{},
		seen:       map[string]*StoredDevice{},
		notSeen:    map[string]*StoredDevice{},
	}
	app.loadManufacturers()

	// Enable BLE interface.
	if err := app.adapter.Enable(); err != nil {
		panic(err)
	}

	go app.checkSeen()
	go app.logic()

	for {
		m := map[string]struct{}{}

		// Start scanning.
		app.loadSeen(m)

		go func() {
			time.Sleep(time.Minute)
			app.adapter.StopScan()
		}()
		println("scanning...")
		if err := app.adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
			uid := device.Address.String()
			if _, ok := m[uid]; ok {
				return
			}
			m[uid] = struct{}{}
			app.discovered <- device
		}); err != nil {
			panic(err)
		}
		println("clean scan exit")
	}
}
