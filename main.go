package main

import (
    "os"
    "fmt"
    "bytes"
    "strings"

    "encoding/hex"

    "github.com/fatih/color"
	"tinygo.org/x/bluetooth"
)

var adapter = bluetooth.DefaultAdapter

func (app *App) markSeen(uid string) {
    addr := fmt.Sprintf(
        "%s:%s:%s",
        string(uid[:2]),
        string(uid[2:4]),
        string(uid[4:6]),
    )
    println("Finding", addr)
    descr, ok := app.db[addr]
    if ok {
        for _, d := range descr {
            color.Green(d)
        }
    }
    app.seen[uid] = true
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

type App struct {
    db map[string][]string
    seen map[string]bool
}

func main() {

    app := &App{
        db: map[string][]string{},
        seen: map[string]bool{},
    }
    app.loadManufacturers()

	// Enable BLE interface.
	must("enable BLE stack", adapter.Enable())

	// Start scanning.
	println("scanning...")

	err := adapter.Scan(func(adapter *bluetooth.Adapter, device bluetooth.ScanResult) {
        uid, err := Parse(device.Address.String())
        if err != nil {
            panic(err)
        }
        uuid := fmt.Sprintf("%X", uid)
		println("found device:", uuid, len(uid), device.RSSI, device.LocalName(), device.Address.String())
        app.markSeen(uuid)
	})
	must("start scan", err)
}

func must(action string, err error) {
	if err != nil {
		panic("failed to " + action + ": " + err.Error())
	}
}

type UUID []byte

// Parse parses a standard-format UUID string, such
// as "1800" or "34DA3AD1-7110-41A1-B1EF-4430F509CDE7".
func Parse(s string) (UUID, error) {
	s = strings.Replace(s, "-", "", -1)
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if err := lenErr(len(b)); err != nil {
		return nil, err
	}
	return UUID(Reverse(b)), nil
}

// lenErr returns an error if n is an invalid UUID length.
func lenErr(n int) error {
	switch n {
	case 2, 16:
		return nil
	}
	return fmt.Errorf("UUIDs must have length 2 or 16, got %d", n)
}

// Reverse returns a reversed copy of u.
func Reverse(u []byte) []byte {
	// Special-case 16 bit UUIDS for speed.
	l := len(u)
	if l == 2 {
		return []byte{u[1], u[0]}
	}
	b := make([]byte, l)
	for i := 0; i < l/2+1; i++ {
		b[i], b[l-i-1] = u[l-i-1], u[i]
	}
	return b
}
