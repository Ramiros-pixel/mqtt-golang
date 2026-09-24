package internal

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	// LampUnknown = belum ada laporan status lampu sama sekali dari device
	LampUnknown = "unknown"
	LampOn      = "on"
	LampOff     = "off"
)

type MQTTCfg struct {
	URL          string
	User         string
	Password     string
	TopicMon     string
	TopicCtrl    string
	TopicShading string
}

// Bridge menjembatani broker MQTT (TCP) dengan frontend:
// subscribe topik monitoring -> simpan ke DB -> broadcast via WebSocket,
// dan publish perintah lampu ke topik controlling.
type Bridge struct {
	cfg      MQTTCfg
	db       *sql.DB
	hub      *Hub
	client   mqtt.Client
	mu       sync.RWMutex
	lampLast string
	// Terakhir yang di-broadcast (untuk dedupe)
	shadingLastMode string
	shadingLastDeg  int
	shadingKnown    bool
	lastSeen        time.Time
}

func NewBridge(cfg MQTTCfg, db *sql.DB, hub *Hub) *Bridge {
	b := &Bridge{cfg: cfg, db: db, hub: hub, lampLast: LampUnknown}

	opts := mqtt.NewClientOptions().
		AddBroker(cfg.URL).
		SetClientID(fmt.Sprintf("smarthome-backend-%d", rand.Intn(1_000_000))).
		SetUsername(cfg.User).
		SetPassword(cfg.Password).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetMaxReconnectInterval(30 * time.Second).
		SetKeepAlive(30 * time.Second).
		SetConnectTimeout(8 * time.Second)

	opts.OnConnect = func(c mqtt.Client) {
		log.Printf("[mqtt] terhubung ke broker %s", cfg.URL)
		if t := c.Subscribe(cfg.TopicMon, 1, b.handleMessage); t.Wait() && t.Error() != nil {
			log.Printf("[mqtt] gagal subscribe %s: %v", cfg.TopicMon, t.Error())
		} else {
			log.Printf("[mqtt] subscribe %s OK", cfg.TopicMon)
		}
	}
	opts.OnConnectionLost = func(_ mqtt.Client, err error) {
		log.Printf("[mqtt] koneksi putus: %v (auto-reconnect aktif)", err)
	}
	opts.OnReconnecting = func(_ mqtt.Client, _ *mqtt.ClientOptions) {
		log.Println("[mqtt] mencoba reconnect...")
	}

	b.client = mqtt.NewClient(opts)
	return b
}

func (b *Bridge) Start() {
	// Connect di goroutine terpisah: dengan SetConnectRetry(true), paho akan
	// terus retry di background sampai broker tersedia, tanpa memblokir startup HTTP.
	go func() {
		if t := b.client.Connect(); t.Wait() && t.Error() != nil {
			log.Printf("[mqtt] broker %s belum bisa dihubungi: %v — retry otomatis tetap aktif", b.cfg.URL, t.Error())
		}
	}()
}

func (b *Bridge) Stop() {
	b.client.Disconnect(250)
}

func (b *Bridge) IsConnected() bool {
	return b.client.IsConnected()
}

// PublishLamp mengirim perintah ON/OFF ke topik controlling.
func (b *Bridge) PublishLamp(on bool) error {
	if !b.client.IsConnected() {
		return fmt.Errorf("belum terhubung ke broker MQTT")
	}
	payload := "OFF"
	if on {
		payload = "ON"
	}
	t := b.client.Publish(b.cfg.TopicCtrl, 1, false, payload)
	// Jangan Wait() tanpa batas — bisa hang selamanya kalau broker tidak merespons.
	if !t.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("timeout: broker tidak mengonfirmasi perintah lampu")
	}
	return t.Error()
}

// PublishShading mengirim perintah shading (manual/auto) ke topik shading.
func (b *Bridge) PublishShading(mode string, degree int) error {
	if !b.client.IsConnected() {
		return fmt.Errorf("belum terhubung ke broker MQTT")
	}
	var payload []byte
	var err error
	if mode == "auto" {
		payload, err = json.Marshal(map[string]string{"mode": "auto"})
	} else {
		payload, err = json.Marshal(map[string]interface{}{"mode": "manual", "degree": degree})
	}
	if err != nil {
		return fmt.Errorf("gagal encode payload: %v", err)
	}
	t := b.client.Publish(b.cfg.TopicShading, 1, false, payload)
	if !t.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("timeout: broker tidak mengonfirmasi perintah shading")
	}
	return t.Error()
}

func (b *Bridge) LampState() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lampLast
}

// ShadingState mengembalikan posisi & mode shading terakhir yang dilaporkan
// ESP32. known=false artinya device belum pernah melapor.
func (b *Bridge) ShadingState() (mode string, degree int, known bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.shadingLastMode, b.shadingLastDeg, b.shadingKnown
}

func (b *Bridge) LastSeen() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastSeen
}

// handleMessage mem-parsing payload JSON dari ESP32, contoh:
// {"temperature": 27.5, "humidity": 62}
// Jika ada field "lamp" (1/0/true/false) dianggap laporan status lampu.
// Jika ada field "shading_degree" dan "shading_mode" dianggap laporan status servo shading.
func (b *Bridge) handleMessage(_ mqtt.Client, msg mqtt.Message) {
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
		log.Printf("[mqtt] payload bukan JSON di %s: %q", msg.Topic(), string(msg.Payload()))
		return
	}

	temp, hasTemp := pickNumber(payload, "temperature", "temp", "suhu")
	hum, hasHum := pickNumber(payload, "humidity", "hum", "kelembaban")

	if hasTemp && hasHum {
		if err := InsertReading(b.db, temp, hum); err != nil {
			log.Printf("[db] gagal simpan reading: %v", err)
		}
		bMsg := map[string]interface{}{
			"type":        "reading",
			"temperature": temp,
			"humidity":    hum,
			"time":        time.Now().UnixMilli(),
		}
		if v, ok := payload["rain_sensor"]; ok {
			if b, ok := v.(bool); ok {
				bMsg["rain_sensor"] = b
			} else if n, ok := v.(float64); ok {
				bMsg["rain_sensor"] = n != 0
			} else if s, ok := v.(string); ok {
				bMsg["rain_sensor"] = strings.EqualFold(strings.TrimSpace(s), "true") || s == "1"
			}
		}
		broadcast(b.hub, bMsg)
	}

	if v, ok := payload["lamp"]; ok {
		state := LampOff
		switch lv := v.(type) {
		case float64:
			if lv != 0 {
				state = LampOn
			}
		case bool:
			if lv {
				state = LampOn
			}
		case string:
			s := strings.ToUpper(strings.TrimSpace(lv))
			if s == "ON" || s == "1" || s == "TRUE" {
				state = LampOn
			}
		}
		b.mu.Lock()
		changed := b.lampLast != state
		b.lampLast = state
		b.mu.Unlock()
		if changed {
			broadcast(b.hub, map[string]interface{}{"type": "lamp_status", "state": state})
		}
	}

// Laporan status servo shading dari ESP32 (di-dedupe: hanya broadcast kalau berubah)
	if _, hasDeg := payload["shading_degree"]; hasDeg {
		if modeVal, hasMode := payload["shading_mode"]; hasMode {
			deg, _ := pickNumber(payload, "shading_degree")
			degInt := int(deg + 0.5)
			mode := ""
			if modeStr, ok := modeVal.(string); ok {
				mode = modeStr
			}
			b.mu.Lock()
			changed := b.shadingLastMode != mode || b.shadingLastDeg != degInt
			if changed {
				b.shadingLastMode = mode
				b.shadingLastDeg = degInt
			}
			b.shadingKnown = true
			b.mu.Unlock()
			if changed {
				broadcast(b.hub, map[string]interface{}{
					"type":   "shading_status",
					"degree": degInt,
					"mode":   mode,
				})
			}
		}
	}

	// Laporan sensor hujan berdiri sendiri (tanpa temp/humidity)
	if v, ok := payload["rain_sensor"]; ok && !(hasTemp && hasHum) {
		rain := false
		switch rv := v.(type) {
		case bool:
			rain = rv
		case float64:
			rain = rv != 0
		case string:
			rain = strings.EqualFold(strings.TrimSpace(rv), "true") || rv == "1"
		}
		broadcast(b.hub, map[string]interface{}{"type": "rain_status", "rain_sensor": rain})
	}

	b.mu.Lock()
	b.lastSeen = time.Now()
	b.mu.Unlock()
}

func pickNumber(m map[string]interface{}, keys ...string) (float64, bool) {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch n := v.(type) {
			case float64:
				return n, true
			case string:
				var f float64
				if _, err := fmt.Sscanf(n, "%f", &f); err == nil {
					return f, true
				}
			}
		}
	}
	return 0, false
}

func broadcast(h *Hub, m map[string]interface{}) {
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	h.Broadcast <- data
}