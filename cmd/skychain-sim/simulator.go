package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/skychain/skychain/pkg/chain"
	"github.com/skychain/skychain/pkg/eventauth"
)

// Simulator coordinates a fleet of virtual devices generating readings.
type Simulator struct {
	cfg     Config
	client  *http.Client
	infoLog *log.Logger
	errLog  *log.Logger
	verbose bool
	devices []*Device
}

// Device represents a virtual IoT device with its own identity and key material.
type Device struct {
	id      string
	privKey ed25519.PrivateKey
	pubKey  ed25519.PublicKey
	sensors []string
	nonce   uint64
	rnd     *rand.Rand
	sim     *Simulator
}

// NewSimulator builds a Simulator from the provided configuration.
func NewSimulator(cfg Config, infoLog, errLog *log.Logger, verbose bool) (*Simulator, error) {
	if infoLog == nil {
		return nil, fmt.Errorf("info logger is required")
	}
	if errLog == nil {
		return nil, fmt.Errorf("error logger is required")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	sim := &Simulator{
		cfg:     cfg,
		client:  client,
		infoLog: infoLog,
		errLog:  errLog,
		verbose: verbose,
	}

	devices, err := sim.createDevices()
	if err != nil {
		return nil, err
	}
	sim.devices = devices

	return sim, nil
}

func (s *Simulator) createDevices() ([]*Device, error) {
	baseRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	devices := make([]*Device, 0, s.cfg.Devices)
	for i := 0; i < s.cfg.Devices; i++ {
		pub, priv, err := ed25519.GenerateKey(nil)
		if err != nil {
			return nil, fmt.Errorf("generate device key: %w", err)
		}

		deviceSensors := selectSensors(baseRand, s.cfg.Sensors)
		dev := &Device{
			id:      fmt.Sprintf("device-%04d", i+1),
			privKey: priv,
			pubKey:  pub,
			sensors: deviceSensors,
			rnd:     rand.New(rand.NewSource(baseRand.Int63())),
			sim:     s,
		}
		devices = append(devices, dev)
	}
	return devices, nil
}

// Run launches device goroutines and blocks until the context is cancelled.
func (s *Simulator) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, dev := range s.devices {
		wg.Add(1)
		go func(d *Device) {
			defer wg.Done()
			d.loop(ctx)
		}(dev)
	}

	<-ctx.Done()
	s.infof("shutdown signal received, waiting for devices to finish")
	wg.Wait()
}

func (d *Device) loop(ctx context.Context) {
	s := d.sim
	s.infof("starting device %s with sensors %v", d.id, d.sensors)

	for {
		interval := d.randomInterval()
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			s.infof("device %s shutting down", d.id)
			return
		case <-timer.C:
		}

		if d.rnd.Float64() < s.cfg.OfflineProbability {
			s.debugf("device %s skipping cycle (offline)", d.id)
			continue
		}

		sensor := d.sensors[int(d.rnd.Int31n(int32(len(d.sensors))))]
		event := d.buildEvent(sensor)

		if err := s.sendEvent(ctx, event); err != nil {
			s.errLog.Printf("device %s send error: %v", d.id, err)
			continue
		}

		s.debugf("device %s sent reading %s nonce=%d", d.id, sensor, event.Nonce)
	}
}

func (d *Device) randomInterval() time.Duration {
	min := d.sim.cfg.MinInterval()
	max := d.sim.cfg.MaxInterval()
	if max <= min {
		return min
	}
	delta := max - min
	return min + time.Duration(d.rnd.Int63n(int64(delta)+1))
}

func (d *Device) buildEvent(sensor string) chain.Event {
	d.nonce++
	ts := time.Now().UTC()
	value, spike := d.generateValue(sensor)
	payload := map[string]any{
		"sensor":     sensor,
		"value":      value,
		"spike":      spike,
		"device_pub": base64.StdEncoding.EncodeToString(d.pubKey),
	}

	evt := chain.Event{
		DeviceID: d.id,
		Nonce:    d.nonce,
		TS:       ts,
		Payload:  payload,
	}
	sig, err := eventauth.Sign(evt, d.privKey)
	if err != nil {
		panic(fmt.Sprintf("sign event: %v", err))
	}
	evt.Signature = sig
	return evt
}

func (d *Device) generateValue(sensor string) (float64, bool) {
	base := 20.0 + d.rnd.Float64()*5
	switch strings.ToLower(sensor) {
	case "temperature":
		base = 15 + d.rnd.Float64()*10
	case "humidity":
		base = 40 + d.rnd.Float64()*20
	case "pressure":
		base = 950 + d.rnd.Float64()*20
	}

	spike := false
	if d.rnd.Float64() < d.sim.cfg.SpikeProbability {
		spike = true
		base *= 1.5 + d.rnd.Float64()
	}

	return base, spike
}

func (s *Simulator) sendEvent(ctx context.Context, event chain.Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.Endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return nil
}

func (s *Simulator) infof(format string, args ...any) {
	s.infoLog.Printf(format, args...)
}

func (s *Simulator) debugf(format string, args ...any) {
	if s.verbose {
		s.infoLog.Printf(format, args...)
	}
}

func selectSensors(rnd *rand.Rand, sensors []string) []string {
	if len(sensors) <= 3 {
		out := make([]string, len(sensors))
		copy(out, sensors)
		return out
	}

	indices := rnd.Perm(len(sensors))
	count := 1 + rnd.Intn(3)
	if count > len(sensors) {
		count = len(sensors)
	}
	chosen := make([]string, 0, count)
	for i := 0; i < count; i++ {
		chosen = append(chosen, sensors[indices[i]])
	}
	return chosen
}
