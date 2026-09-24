/*
 * ESP32 Smart Home — integrasi penuh dengan backend Go (mqttintegrate)
 * --------------------------------------------------------------------
 * Peran firmware:
 *   1. Membaca sensor suhu/kelembaban (DHT11/DHT22) dan sensor hujan.
 *   2. Menggerakkan servo shading (tirai) 0-180 derajat.
 *   3. Mengontrol lampu via relay.
 *   4. Menerima perintah dari dashboard lewat MQTT:
 *        - topik controlling : teks "ON"/"OFF"          -> lampu
 *        - topik shading     : {"mode":"manual","degree":N}
 *                              {"mode":"auto"}
 *   5. Melapor balik ke backend lewat topik monitoring (JSON).
 *
 * Logika MODE OTOMATIS (dijalankan di ESP32, bukan di backend):
 *   - rain_sensor == true  -> servo tutup penuh  (0 derajat)
 *   - rain_sensor == false -> servo terbuka penuh (180 derajat)
 *
 * Koneksi MQTT butuh autentikasi (username/password).
 *
 * LIBRARY YANG WAJIB DIINSTALL (menu Sketch -> Include Library -> Manage Libraries):
 *   - "PubSubClient"  oleh Nick O'Leary     (MQTT)
 *   - "DHT sensor library" oleh Adafruit    (sensor suhu/kelembaban)
 *   - "Adafruit Unified Sensor"             (dependensi DHT)
 *   - "ESP32Servo"  (jika servo tidak jalan dengan library Servo bawaan)
 *
 * PINTERKONFIRMASI PIN -> sesuaikan di bagian SETUP PIN DI BAWAH.
 */

#include <WiFi.h>
#include <PubSubClient.h>
#include <DHT.h>
#include <Servo.h>

// ================== KONFIGURASI JARINGAN ==================
const char* WIFI_SSID     = "rina";
const char* WIFI_PASSWORD = "Haribasa";

// ================== KONFIGURASI MQTT ==================
// Harus SAMA dengan backend/config.go
const char* MQTT_BROKER   = "100.78.90.71";
const int   MQTT_PORT     = 1883;
const char* MQTT_USER     = "ramiros";
const char* MQTT_PASSWORD = "bagas_1911";

// Topik — HARUS cocok dengan backend
const char* TOPIC_MONITORING  = "esp32/monitor/monitoring";   // ESP32 -> backend
const char* TOPIC_CONTROLLING = "esp32/monitor/controlling";  // backend -> ESP32 (lampu)
const char* TOPIC_SHADING     = "esp32/monitor/shading";      // backend -> ESP32 (servo)

const char* MQTT_CLIENT_ID  = "esp32-smarthome-01";

// ================== SETUP PIN ==================
const int PIN_DHT      = D4;   // Data pin DHT11/DHT22
#define DHT_TYPE       DHT11   // ubah ke DHT22 kalau pakai AM2302
const int PIN_RAIN     = D5;   // Output DIGITAL sensor hujan
const int PIN_SERVO    = D13;  // Pin PWM servo shading
const int PIN_LAMP     = D27;  // Pin relay lampu

// Jika output digital sensor hujan terbalik (LOW saat hujan), set true.
// Kebanyakan modul hujan digital: LOW(0) = kena air/hujan.
const bool RAIN_INVERT = false;

// ================== PARAMETER ==================
const long PUBLISH_INTERVAL_MS = 5000;   // lapor ke backend tiap 5 detik
const long SERVO_MOVE_STEP_MS  = 30;     // jeda antar-langkah servo (halus)
const int  DEGREE_CLOSED       = 0;      // posisi tutup penuh
const int  DEGREE_OPEN         = 180;    // posisi buka penuh

// ================== OBJEK GLOBAL ==================
WiFiClient   espClient;
PubSubClient mqtt(espClient);
DHT          dht(PIN_DHT, DHT_TYPE);
Servo        shadeServo;

// ================== STATE ==================
bool     mqttConnected  = false;
unsigned long lastPublish = 0;

// Lampu
bool     lampOn   = false;

// Shading
String   shadingMode   = "manual";   // "manual" | "auto"
int      shadeDegree   = DEGREE_OPEN; // posisi servo saat ini (0-180)
int      shadeTarget   = DEGREE_OPEN; // target yang sedang dikejar
unsigned long lastStep = 0;          // jeda langkah servo

// Buffer perintah yang menunggu diproses di loop (MQTT callback jangan
// memanggil digitalWrite/move langsung agar aman di semua platform)
volatile bool   pendingLampCmd   = false;
volatile bool   pendingLampValue = false;
volatile bool   pendingShading   = false;
volatile bool   pendingShadeAuto = false;
volatile int    pendingShadeDeg  = DEGREE_OPEN;

// ================== CALLBACK MQTT ==================
void mqttCallback(char* topic, byte* payload, unsigned int length) {
  // Baca payload ke string yang aman
  String msg;
  for (unsigned int i = 0; i < length; i++) msg += (char)payload[i];
  msg.trim();

  Serial.printf("[MQTT] %s : %s\n", topic, msg.c_str());

  if (String(topic) == TOPIC_CONTROLLING) {
    // Perintah lampu: "ON" / "OFF"
    pendingLampValue = (msg.equalsIgnoreCase("ON") || msg == "1");
    pendingLampCmd   = true;
  }
  else if (String(topic) == TOPIC_SHADING) {
    // Perintah shading, contoh:
    //   {"mode":"manual","degree":90}
    //   {"mode":"auto"}
    String lower = msg;
    lower.toLowerCase();
    bool handled = false;
    if (lower.indexOf("\"auto\"") >= 0) {
      pendingShadeAuto = true;
      handled = true;
    } else if (lower.indexOf("\"manual\"") >= 0) {
      // ambil nilai "degree": 123
      int a = msg.indexOf("\"degree\"");
      if (a >= 0) {
        a = msg.indexOf(':', a);
        int b = msg.indexOf(',', a);
        int e = msg.indexOf('}', a);
        int end = (b >= 0 && b < e) ? b : e;
        String num = msg.substring(a + 1, end);
        num.trim();
        int deg = num.toInt();
        if (deg < DEGREE_CLOSED) deg = DEGREE_CLOSED;
        if (deg > DEGREE_OPEN)   deg = DEGREE_OPEN;
        pendingShadeDeg  = deg;
        pendingShading   = true;
        handled = true;
      }
    }
    if (!handled) Serial.println("[SHADING] payload tidak dikenali");
  }
}

// ================== JARINGAN & MQTT ==================
void connectWiFi() {
  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  Serial.print("Menyambung WiFi");
  unsigned long t0 = millis();
  while (WiFi.status() != WL_CONNECTED) {
    delay(400);
    Serial.print(".");
    if (millis() - t0 > 20000) { Serial.println(" timeout, coba lagi"); return; }
  }
  Serial.println();
  Serial.printf("WiFi OK, IP: %s\n", WiFi.localIP().toString().c_str());
}

bool connectMQTT() {
  if (mqtt.connected()) return true;
  Serial.printf("Menyambung MQTT %s:%d ...\n", MQTT_BROKER, MQTT_PORT);
  // Client ID unik agar tidak bentrok antar-restart (max 23 char)
  String cid = String(MQTT_CLIENT_ID);
  int rc = mqtt.connect(cid.c_str(), MQTT_USER, MQTT_PASSWORD);
  if (rc != 1) {
    Serial.printf("MQTT gagal, kode=%d\n", rc);
    return false;
  }
  mqttConnected = true;
  Serial.println("MQTT terhubung");
  // Subscribe kedua topik perintah
  mqtt.subscribe(TOPIC_CONTROLLING, 1);
  mqtt.subscribe(TOPIC_SHADING, 1);
  Serial.printf("Subscribe: %s | %s\n", TOPIC_CONTROLLING, TOPIC_SHADING);
  // Lapor status awal secepatnya
  publishMonitoring();
  return true;
}

// ================== AKTUATOR ==================
void setLamp(bool on) {
  if (lampOn == on) return;
  lampOn = on;
  digitalWrite(PIN_LAMP, lampOn ? HIGH : LOW);
  Serial.printf("[LAMP] %s\n", lampOn ? "ON" : "OFF");
}

// Gerakkan servo secara bertahap agar tidak kasar/nyentak.
// Dipanggil tiap loop; menggerakkan 1 langkah menuju target.
void stepServo() {
  if (shadeDegree == shadeTarget) return;
  if (millis() - lastStep < SERVO_MOVE_STEP_MS) return;
  lastStep = millis();
  if (shadeDegree < shadeTarget) shadeDegree++;
  else shadeDegree--;
  shadeServo.write(shadeDegree);
}

// Set target servo (mode manual)
void setShadeTarget(int deg, String mode) {
  if (deg < DEGREE_CLOSED) deg = DEGREE_CLOSED;
  if (deg > DEGREE_OPEN)   deg = DEGREE_OPEN;
  shadeTarget = deg;
  shadingMode = mode;
  Serial.printf("[SHADING] mode=%s target=%d\n", mode.c_str(), deg);
}

// Evaluasi ulang mode auto berdasarkan sensor hujan
void applyAutoLogic(bool raining) {
  if (shadingMode != "auto") return;
  int desired = raining ? DEGREE_CLOSED : DEGREE_OPEN;
  if (desired != shadeTarget) {
    setShadeTarget(desired, "auto");
  }
}

// ================== SENSOR Hujan ==================
bool readRain() {
  int raw = digitalRead(PIN_RAIN);
  bool raining = raw == LOW;      // LOW = ada air (umum)
  if (RAIN_INVERT) raining = !raining;
  return raining;
}

// ================== PUBLISH MONITORING ==================
void publishMonitoring() {
  if (!mqtt.connected()) return;

  float temp = dht.readTemperature();
  float hum  = dht.readHumidity();
  if (isnan(temp)) temp = 0;   // DHT gagal baca -> kirim 0, jangan "nan"
  if (isnan(hum))  hum  = 0;
  bool  rain = readRain();

  // Update auto-logic tiap kali ada reading baru
  applyAutoLogic(rain);

  // Build JSON manual (hindari dependensi ArduinoJson, lebih ringan)
  String lampStr = lampOn ? "ON" : "OFF";
  String payload = "{\"temperature\":";
  payload += String(temp, 1);
  payload += ",\"humidity\":";
  payload += String(hum, 0);
  payload += ",\"lamp\":\"";
  payload += lampStr;
  payload += "\",\"rain_sensor\":";
  payload += (rain ? "true" : "false");
  payload += ",\"shading_degree\":";
  payload += String(shadeDegree);
  payload += ",\"shading_mode\":\"";
  payload += shadingMode;
  payload += "\"}";

  mqtt.publish(TOPIC_MONITORING, payload.c_str());
  Serial.printf("[PUB] %s\n", payload.c_str());
}

// ================== SETUP ==================
void setup() {
  Serial.begin(115200);
  delay(300);
  Serial.println("\n=== ESP32 Smart Home (Shading + Hujan) ===");

  pinMode(PIN_LAMP, OUTPUT);
  pinMode(PIN_RAIN, INPUT);
  digitalWrite(PIN_LAMP, LOW); // mulai mati

  dht.begin();

  shadeServo.attach(PIN_SERVO);
  shadeServo.write(shadeDegree);
  delay(500); // kasih waktu servo stabil setelah boot

  connectWiFi();

  mqtt.setCallback(mqttCallback);
  connectMQTT();
}

// ================== LOOP ==================
void loop() {
  // Jaga koneksi
  if (WiFi.status() != WL_CONNECTED) {
    Serial.println("WiFi putus, connect ulang");
    connectWiFi();
  }
  if (!mqtt.connected()) {
    static unsigned long lastTry = 0;
    if (millis() - lastTry > 5000) { lastTry = millis(); connectMQTT(); }
  }

  mqtt.loop();

  // ---- Proses perintah lampu (aman di luar callback) ----
  if (pendingLampCmd) {
    pendingLampCmd = false;
    setLamp(pendingLampValue);
  }

  // ---- Proses perintah shading ----
  if (pendingShadeAuto) {
    pendingShadeAuto = false;
    shadingMode = "auto";
    Serial.println("[SHADING] beralih ke mode AUTO");
    applyAutoLogic(readRain()); // langsung evaluasi sekarang
  }
  if (pendingShading) {
    pendingShading = false;
    setShadeTarget(pendingShadeDeg, "manual");
  }

  // ---- Gerakkan servo menuju target (berjalan di loop) ----
  stepServo();

  // ---- Lapor ke backend secara berkala ----
  if (mqtt.connected() && millis() - lastPublish > PUBLISH_INTERVAL_MS) {
    lastPublish = millis();
    publishMonitoring();
  }
}
