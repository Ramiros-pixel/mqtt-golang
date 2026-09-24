package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPPort     int
	DBHost       string
	DBPort       int
	DBUser       string
	DBPass       string
	DBName       string
	MQTTBroker   string
	MQTTPort     int
	MQTTUser     string
	MQTTPass     string
	TopicMon     string
	TopicCtrl    string
	TopicShading string
	JWTSecret    string
	TokenExpiry  time.Duration
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func LoadConfig() *Config {
	return &Config{
		HTTPPort:     envInt("HTTP_PORT", 8080),
		DBHost:       envStr("DB_HOST", "127.0.0.1"),
		DBPort:       envInt("DB_PORT", 3306),
		DBUser:       envStr("DB_USER", "root"),
		DBPass:       envStr("DB_PASS", ""),
		DBName:       envStr("DB_NAME", "smarthome"),
		MQTTBroker:   envStr("MQTT_BROKER", "100.78.90.71"),
		MQTTPort:     envInt("MQTT_PORT", 1883),
		MQTTUser:     envStr("MQTT_USER", "ramiros"),
		MQTTPass:     envStr("MQTT_PASS", "bagas_1911"),
		TopicMon:     envStr("TOPIC_MONITORING", "esp32/monitor/monitoring"),
		TopicCtrl:    envStr("TOPIC_CONTROLLING", "esp32/monitor/controlling"),
		TopicShading: envStr("TOPIC_SHADING", "esp32/monitor/shading"),
		JWTSecret:    envStr("JWT_SECRET", "smarthome-secret-bagas-1911"),
		TokenExpiry:  24 * time.Hour,
	}
}

func (c *Config) MySQLDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		c.DBUser, c.DBPass, c.DBHost, c.DBPort, c.DBName)
}

func (c *Config) MQTTURL() string {
	return fmt.Sprintf("tcp://%s:%d", c.MQTTBroker, c.MQTTPort)
}