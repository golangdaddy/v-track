package main

import (
    "os"
    "time"
)

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
                b, err := app.Marshal(sd)
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
                err := app.Unmarshal(b, sd)
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
            b, err = app.Marshal(sd)
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
