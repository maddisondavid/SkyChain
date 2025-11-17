package eventauth

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/skychain/skychain/pkg/chain"
)

func TestCanonicalEventMessageDeterministic(t *testing.T) {
	ts := time.Date(2024, time.January, 2, 15, 4, 5, 123000000, time.UTC)
	evt := chain.Event{
		DeviceID: "sensor-1",
		Nonce:    42,
		TS:       ts,
		Payload: map[string]any{
			"b": 1,
			"a": map[string]any{
				"z": 2,
				"y": 3,
			},
		},
	}

	msg, err := CanonicalEventMessage(evt)
	if err != nil {
		t.Fatalf("canonical message: %v", err)
	}
	expected := `{"device_id":"sensor-1","nonce":42,"payload":{"a":{"y":3,"z":2},"b":1},"ts":"2024-01-02T15:04:05.123Z"}`
	if string(msg) != expected {
		t.Fatalf("unexpected canonical output: %s", msg)
	}
}

func TestVerify(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	evt := chain.Event{
		DeviceID: "sensor-1",
		Nonce:    1,
		TS:       time.Unix(100, 0).UTC(),
		Payload:  map[string]any{"temp": 12.3},
	}

	sig, err := Sign(evt, priv)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	evt.Signature = sig

	if err := Verify(evt, pub); err != nil {
		t.Fatalf("verify valid signature: %v", err)
	}

	evt.Signature = "invalid"
	if err := Verify(evt, pub); err == nil {
		t.Fatalf("expected invalid signature error")
	}
}
