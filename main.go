package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include "bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"
)

//go:embed ui.html
var uiHTML []byte

const (
	apiKey = "v6GFvkweNo7DK7yD3ylIZ9w52aKBU0eJ7wLXkSR3"
	host   = "https://api-kic.lgthinq.com"
)

type Config struct {
	PatToken    string `json:"pat_token"`
	CountryCode string `json:"country_code"`
	ClientID    string `json:"client_id"`
}

type DeviceInfo struct {
	Alias      string `json:"alias"`
	DeviceType string `json:"deviceType"`
	ModelName  string `json:"modelName"`
	Reportable bool   `json:"reportable"`
}

type RawDevice struct {
	DeviceID   string     `json:"deviceId"`
	DeviceInfo DeviceInfo `json:"deviceInfo"`
}

type DeviceListResponse struct {
	Response []RawDevice `json:"response"`
}

type ACDevice struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Model       string  `json:"model"`
	IsOnline    bool    `json:"isOnline"`
	IsOn        bool    `json:"isOn"`
	TargetTemp  int     `json:"targetTemp"`
	CurrentTemp float64 `json:"currentTemp,omitempty"`
	Mode        string  `json:"mode"`
}

type Client struct {
	config     Config
	httpClient *http.Client
	ctrlMu     sync.Mutex
}

func randomUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func configFilePath() string {
	home, err := os.UserHomeDir()
	if err == nil {
		dir := filepath.Join(home, ".config", "lg-ac-menubar")
		os.MkdirAll(dir, 0755)
		return filepath.Join(dir, "config.json")
	}
	return "config.json"
}

func LoadConfig() Config {
	var cfg Config
	paths := []string{
		configFilePath(),
		"config.json",
	}

	for _, p := range paths {
		if data, err := os.ReadFile(p); err == nil {
			if err := json.Unmarshal(data, &cfg); err == nil && cfg.PatToken != "" {
				return cfg
			}
		}
	}
	return Config{CountryCode: "IN", ClientID: randomUUID()}
}

func SaveConfig(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFilePath(), data, 0600)
}

func NewClient(cfg Config) *Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Transport: tr,
			Timeout:   15 * time.Second,
		},
	}
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.config.PatToken)
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("x-client-id", c.config.ClientID)
	req.Header.Set("x-country", c.config.CountryCode)
	req.Header.Set("x-country-code", c.config.CountryCode)
	req.Header.Set("country", c.config.CountryCode)
	req.Header.Set("x-message-id", randomUUID())
	req.Header.Set("x-service-phase", "OP")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

func (c *Client) GetACDevices() ([]ACDevice, error) {
	req, err := http.NewRequest("GET", host+"/devices", nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var res DeviceListResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}

	var acList []ACDevice
	for _, raw := range res.Response {
		if strings.Contains(strings.ToUpper(raw.DeviceInfo.DeviceType), "AIR_CONDITIONER") ||
			strings.Contains(strings.ToUpper(raw.DeviceInfo.DeviceType), "AC") {

			name := raw.DeviceInfo.Alias
			if name == "" {
				name = raw.DeviceInfo.ModelName
			}

			ac := ACDevice{
				ID:          raw.DeviceID,
				Name:        name,
				Model:       raw.DeviceInfo.ModelName,
				IsOnline:    false,
				IsOn:        false,
				TargetTemp:  24,
				CurrentTemp: 0,
				Mode:        "COOL",
			}
			acList = append(acList, ac)
		}
	}

	var wg sync.WaitGroup
	for i := range acList {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			state, isOnline, err := c.GetDeviceState(acList[idx].ID)
			if err == nil && isOnline {
				acList[idx].IsOnline = true
				c.populateState(&acList[idx], state)
			} else {
				acList[idx].IsOnline = false
			}
		}(i)
	}
	wg.Wait()

	return acList, nil
}

func (c *Client) GetDeviceState(deviceID string) (map[string]interface{}, bool, error) {
	req, err := http.NewRequest("GET", host+"/devices/"+deviceID+"/state", nil)
	if err != nil {
		return nil, false, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}

	if resp.StatusCode == 416 || resp.StatusCode == 404 {
		// Device is offline / not connected to cloud
		return nil, false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, false, err
	}

	if resMap, ok := raw["response"].(map[string]interface{}); ok {
		return resMap, true, nil
	}
	if resMap, ok := raw["result"].(map[string]interface{}); ok {
		return resMap, true, nil
	}
	return raw, true, nil
}

func (c *Client) populateState(ac *ACDevice, state map[string]interface{}) {
	if op, ok := state["operation"].(map[string]interface{}); ok {
		if mode, ok := op["airConOperationMode"].(string); ok {
			ac.IsOn = strings.EqualFold(mode, "POWER_ON")
		}
	}

	if job, ok := state["airConJobMode"].(map[string]interface{}); ok {
		if mode, ok := job["currentJobMode"].(string); ok {
			ac.Mode = mode
		}
	}

	if temp, ok := state["temperature"].(map[string]interface{}); ok {
		if target, ok := temp["targetTemperature"].(float64); ok && target > 0 {
			ac.TargetTemp = int(target)
		} else if targetC, ok := temp["targetTemperatureC"].(float64); ok && targetC > 0 {
			ac.TargetTemp = int(targetC)
		}
		if curr, ok := temp["currentTemperature"].(float64); ok && curr > 0 {
			ac.CurrentTemp = curr
		}
	}
}

func (c *Client) SetPower(deviceID string, on bool) error {
	mode := "POWER_ON"
	if !on {
		mode = "POWER_OFF"
	}

	payload := map[string]interface{}{
		"operation": map[string]interface{}{
			"airConOperationMode": mode,
		},
	}
	return c.postControl(deviceID, payload)
}

func (c *Client) SetTemperature(deviceID string, temp int) error {
	payload := map[string]interface{}{
		"temperature": map[string]interface{}{
			"targetTemperature": temp,
		},
	}
	return c.postControl(deviceID, payload)
}

func (c *Client) postControl(deviceID string, payload map[string]interface{}) error {
	c.ctrlMu.Lock()
	defer c.ctrlMu.Unlock()

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", host+"/devices/"+deviceID+"/control", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var errData struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &errData) == nil && errData.Error.Code != "" {
			if errData.Error.Code == "1222" {
				return fmt.Errorf("AC is offline (switched off at breaker or Wi-Fi disconnected)")
			}
			if errData.Error.Code == "1104" {
				return fmt.Errorf("AC is busy processing. Please wait a moment")
			}
			return fmt.Errorf("%s (code %s)", errData.Error.Message, errData.Error.Code)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// Server Component
type Server struct {
	client    *Client
	listener  net.Listener
	mu        sync.Mutex
	cachedACs []ACDevice
	cfg       Config
	Port      int
}

type SetupRequest struct {
	PatToken    string `json:"pat_token"`
	CountryCode string `json:"country_code"`
}

type PowerRequest struct {
	DeviceID string `json:"deviceId"`
	On       bool   `json:"on"`
}

type TempRequest struct {
	DeviceID   string `json:"deviceId"`
	TargetTemp int    `json:"targetTemp"`
}

func NewServer(cfg Config) (*Server, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	port := listener.Addr().(*net.TCPAddr).Port
	s := &Server{
		cfg:      cfg,
		client:   NewClient(cfg),
		listener: listener,
		Port:     port,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/clipboard", s.handleClipboard)
	mux.HandleFunc("/api/open-portal", s.handleOpenPortal)
	mux.HandleFunc("/api/setup", s.handleSetup)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/devices", s.handleGetDevices)
	mux.HandleFunc("/api/power", s.handlePower)
	mux.HandleFunc("/api/temperature", s.handleTemperature)
	mux.HandleFunc("/api/quit", s.handleQuit)

	go http.Serve(listener, mux)
	return s, nil
}

func (s *Server) GetURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d/", s.Port)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(uiHTML)
}

func (s *Server) handleClipboard(w http.ResponseWriter, r *http.Request) {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"text": strings.TrimSpace(string(out))})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.mu.Lock()
	isConfigured := s.cfg.PatToken != ""
	country := s.cfg.CountryCode
	s.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"isConfigured": isConfigured,
		"country":      country,
	})
}

func (s *Server) handleOpenPortal(w http.ResponseWriter, r *http.Request) {
	_ = exec.Command("open", "https://connect-pat.lgthinq.com").Start()
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req.PatToken = strings.TrimSpace(req.PatToken)
	req.CountryCode = strings.ToUpper(strings.TrimSpace(req.CountryCode))
	if req.CountryCode == "" {
		req.CountryCode = "IN"
	}

	if req.PatToken == "" {
		http.Error(w, "Token cannot be empty", http.StatusBadRequest)
		return
	}

	// Test credentials
	testCfg := Config{
		PatToken:    req.PatToken,
		CountryCode: req.CountryCode,
		ClientID:    randomUUID(),
	}
	testClient := NewClient(testCfg)

	devices, err := testClient.GetACDevices()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to connect: %v", err), http.StatusBadRequest)
		return
	}

	// Save and update
	s.mu.Lock()
	s.cfg = testCfg
	s.client = testClient
	s.cachedACs = devices
	s.mu.Unlock()

	SaveConfig(testCfg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"count":   len(devices),
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.cfg.PatToken = ""
	s.cachedACs = nil
	s.client = NewClient(s.cfg)
	s.mu.Unlock()

	SaveConfig(s.cfg)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGetDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.mu.Lock()
	if s.cfg.PatToken == "" {
		s.mu.Unlock()
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	s.mu.Unlock()

	devices, err := s.client.GetACDevices()
	if err != nil {
		s.mu.Lock()
		cached := s.cachedACs
		s.mu.Unlock()
		if len(cached) > 0 {
			json.NewEncoder(w).Encode(cached)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	s.cachedACs = devices
	s.mu.Unlock()

	json.NewEncoder(w).Encode(devices)
}

func (s *Server) handlePower(w http.ResponseWriter, r *http.Request) {
	var req PowerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.client.SetPower(req.DeviceID, req.On); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	for i := range s.cachedACs {
		if s.cachedACs[i].ID == req.DeviceID {
			s.cachedACs[i].IsOn = req.On
		}
	}
	s.mu.Unlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleTemperature(w http.ResponseWriter, r *http.Request) {
	var req TempRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.client.SetTemperature(req.DeviceID, req.TargetTemp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.mu.Lock()
	for i := range s.cachedACs {
		if s.cachedACs[i].ID == req.DeviceID {
			s.cachedACs[i].TargetTemp = req.TargetTemp
		}
	}
	s.mu.Unlock()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (s *Server) handleQuit(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	go func() {
		os.Exit(0)
	}()
}

func main() {
	cfg := LoadConfig()

	srv, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to start internal server: %v", err)
	}

	fmt.Printf("LG AC Menu Bar ready at: %s\n", srv.GetURL())
	fmt.Println("Launching macOS Menu Bar item...")

	cURL := C.CString(srv.GetURL())
	defer C.free(unsafe.Pointer(cURL))
	C.StartMenuBar(cURL)
}
