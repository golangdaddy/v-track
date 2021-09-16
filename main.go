package main

import (
    "os"
    "fmt"
    "sync"
    "time"
    "sort"
    "bytes"
    "strings"

    "io/fs"
    "path/filepath"
    "encoding/json"

    "github.com/fatih/color"
	"tinygo.org/x/bluetooth"
)

type StoredDevice struct {
    UID string
    LastSeen time.Time
    LocalName string
    CustomName string
    Seen int
}

type App struct {
    adapter *bluetooth.Adapter
    discovered chan bluetooth.ScanResult
    db map[string][]string
    seen map[string]*StoredDevice
    notSeen map[string]*StoredDevice
    sync.RWMutex
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

func (app *App) logic() {
    for {
        select {
        case device := <- app.discovered:

            uid := device.Address.String()

            sd := &StoredDevice{
                UID: uid,
                CustomName: "?",
                LocalName: device.LocalName(),
            }

            if len(sd.LocalName) > 0 {
                println("skip device:", uid, device.RSSI, device.LocalName(), device.Address.String())
                b, err := json.Marshal(sd)
                if err != nil {
                    panic(err)
                }
                if err := os.WriteFile("./skipped/"+uid, b, 0666); err != nil {
                    panic(err)
                }
                continue
            }

            b, err := os.ReadFile("./seen/"+uid)
            if err == nil {
                // open existing file
                err := json.Unmarshal(b, sd)
                if err != nil {
                    panic(err)
                }
                sd.Seen++
            } else {
                println("found device:", uid, device.RSSI, device.LocalName(), device.Address.String())
            }

            // set correct time on the record
            sd.LastSeen = time.Now().UTC()
            d := time.Since(sd.LastSeen)
            if d > (10 * time.Minute) {
                println("THEY HAVE COME BACK TO US", d.String())
            }

            app.see(uid, sd)

            // flush to file
            b, err = json.Marshal(sd)
            if err != nil {
                panic(err)
            }
            if err := os.WriteFile("seen/"+uid, b, 0666); err != nil {
                panic(err)
            }

        //case <- time.NewTicker(time.Second).C: println("ticker")

        }
    }
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
            func (j, k int) bool {
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
            if err := json.Unmarshal(b, sd); err != nil {
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
        adapter: bluetooth.DefaultAdapter,
        discovered: make(chan bluetooth.ScanResult, 10),
        db: map[string][]string{},
        seen: map[string]*StoredDevice{},
        notSeen: map[string]*StoredDevice{},
    }
    //app.loadManufacturers()
    go app.checkSeen()
    go app.logic()

	// Enable BLE interface.
	if err := app.adapter.Enable(); err != nil {
        panic(err)
    }

    for {
        m := map[string]struct{}{}

    	// Start scanning.
        app.loadSeen(m)

        go func () {
            time.Sleep(time.Minute)
            app.adapter.StopScan()
        }()
    	println("scanning...")
    	if err := app.adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
            uid := device.Address.String()
            if _, ok := m[uid]; ok {
                println("skip scan exit")
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
