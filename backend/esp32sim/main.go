// Simulasi ESP32: publish payload ke esp32/monitor/monitoring untuk uji end-to-end.
// Pemakaian: go run . [mode]
//   mode: full (default) — satu payload lengkap + shading_status terpisah
//   auto — simulasi servo bergerak di mode auto (buka 180, lalu hujan → tutup 0)
package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const broker = "tcp://100.78.90.71:1883"
const mUser = "ramiros"
const mPass = "bagas_1911"
const topicMon = "esp32/monitor/monitoring"

func main() {
	mode := "full"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(fmt.Sprintf("esp32-sim-%d", rand.Intn(1_000_000))).
		SetUsername(mUser).
		SetPassword(mPass).
		SetConnectTimeout(10 * time.Second)
	client := mqtt.NewClient(opts)
	t := client.Connect()
	if !t.Wait() || t.Error() != nil {
		fmt.Println("GALAI connect broker:", t.Error())
		os.Exit(1)
	}
	defer client.Disconnect(100)

	pub := func(m map[string]interface{}) {
		b, _ := json.Marshal(m)
		pt := client.Publish(topicMon, 1, false, b)
		if !pt.WaitTimeout(5*time.Second) || pt.Error() != nil {
			fmt.Println("GAGAL publish:", string(b), pt.Error())
			return
		}
		fmt.Println("PUBLISH:", string(b))
	}

	switch mode {
	case "auto":
		// Mode auto, tidak hujan → servo terbuka 180°
		pub(map[string]interface{}{
			"temperature": 28.5, "humidity": 65, "lamp": "OFF",
			"rain_sensor": false, "shading_mode": "auto", "shading_degree": 180,
		})
		time.Sleep(1 * time.Second)
		// Hujan datang → servo ditutup 0°
		pub(map[string]interface{}{
			"temperature": 27.0, "humidity": 78, "lamp": "ON",
			"rain_sensor": true, "shading_mode": "auto", "shading_degree": 0,
		})
		time.Sleep(1 * time.Second)
		// Hujan berhenti → servo dibuka lagi 180°
		pub(map[string]interface{}{
			"temperature": 26.5, "humidity": 70, "lamp": "ON",
			"rain_sensor": false, "shading_mode": "auto", "shading_degree": 180,
		})
	case "manual":
		// Mode manual di 90°
		pub(map[string]interface{}{
			"temperature": 27.5, "humidity": 62, "lamp": "ON",
			"rain_sensor": false, "shading_mode": "manual", "shading_degree": 90,
		})
	case "full":
		pub(map[string]interface{}{
			"temperature": 27.5, "humidity": 62, "lamp": "ON",
			"rain_sensor": false, "shading_mode": "manual", "shading_degree": 120,
		})
	default:
		fmt.Println("mode tidak dikenal:", mode)
	}
}
