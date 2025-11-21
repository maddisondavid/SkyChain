package node

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/skychain/skychain/pkg/chain"
	"github.com/skychain/skychain/pkg/eventauth"
	"github.com/skychain/skychain/pkg/registry"
)

type stubSink struct {
	mu     sync.Mutex
	blocks []chain.Block
}

func (s *stubSink) PersistBlock(block chain.Block) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blocks = append(s.blocks, block)
	return nil
}

func TestHandleEventSignatureValidation(t *testing.T) {
	dir := t.TempDir()
	chainStore, err := chain.NewChain("validator", "secret")
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}
	sink := &stubSink{}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	devicesPath := filepath.Join(dir, "devices.json")
	payload := []byte(`{"device-1":"` + base64.StdEncoding.EncodeToString(pub) + `"}`)
	if err := os.WriteFile(devicesPath, payload, 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	reg, err := registry.Load(devicesPath)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}

	node, err := NewNode(chainStore, sink, time.Second, reg)
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	baseEvent := chain.Event{
		DeviceID: "device-1",
		Nonce:    1,
		TS:       time.Unix(100, 0).UTC(),
		Payload:  map[string]any{"temp": 42},
	}
	sig, err := eventauth.Sign(baseEvent, priv)
	if err != nil {
		t.Fatalf("sign event: %v", err)
	}
	baseEvent.Signature = sig

	send := func(body map[string]any) *httptest.ResponseRecorder {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		req := httptest.NewRequest(http.MethodPost, "/event", bytes.NewReader(data))
		rr := httptest.NewRecorder()
		node.handleEvent(rr, req)
		return rr
	}

	body := map[string]any{
		"device_id": baseEvent.DeviceID,
		"nonce":     baseEvent.Nonce,
		"ts":        baseEvent.TS.Format(time.RFC3339Nano),
		"payload":   baseEvent.Payload,
		"sig":       "invalid",
	}
	if resp := send(body); resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid signature got %d", resp.Code)
	}

	body["sig"] = baseEvent.Signature
	if resp := send(body); resp.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for valid event got %d", resp.Code)
	}

	if resp := send(body); resp.Code != http.StatusConflict {
		t.Fatalf("expected 409 for replayed nonce got %d", resp.Code)
	}
}

func TestHandleProofLookup(t *testing.T) {
	dir := t.TempDir()
	chainStore, err := chain.NewChain("validator", "secret")
	if err != nil {
		t.Fatalf("new chain: %v", err)
	}
	sink := &stubSink{}

	regPath := filepath.Join(dir, "devices.json")
	if err := os.WriteFile(regPath, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write registry: %v", err)
	}
	reg, err := registry.Load(regPath)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}

	node, err := NewNode(chainStore, sink, time.Second, reg)
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	evt := chain.Event{DeviceID: "dev-1", Nonce: 1, TS: time.Unix(10, 0).UTC()}
	blk, err := chainStore.AppendBlock([]chain.Event{evt})
	if err != nil {
		t.Fatalf("append block: %v", err)
	}
	txid, err := chain.EventHash(evt)
	if err != nil {
		t.Fatalf("event hash: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/proof?txid="+txid, nil)
	rr := httptest.NewRecorder()
	node.handleProof(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", rr.Code)
	}
	var proof chain.MerkleProof
	if err := json.NewDecoder(rr.Body).Decode(&proof); err != nil {
		t.Fatalf("decode proof: %v", err)
	}
	if proof.BlockIndex != blk.Index || proof.TxID != txid {
		t.Fatalf("unexpected proof metadata")
	}
	if err := chain.VerifyProof(proof); err != nil {
		t.Fatalf("verify proof: %v", err)
	}

	missingReq := httptest.NewRequest(http.MethodGet, "/proof?txid=deadbeef", nil)
	missing := httptest.NewRecorder()
	node.handleProof(missing, missingReq)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing tx got %d", missing.Code)
	}
}
