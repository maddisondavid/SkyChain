package eventauth

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/skychain/skychain/pkg/chain"
)

// ErrInvalidSignature indicates that the provided signature does not match the
// event payload and public key combination.
var ErrInvalidSignature = errors.New("invalid event signature")

// Sign generates a base64-encoded ED25519 signature for the supplied event
// using the provided private key.
func Sign(evt chain.Event, privateKey ed25519.PrivateKey) (string, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return "", errors.New("ed25519 private key required")
	}

	msg, err := CanonicalEventMessage(evt)
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(privateKey, msg)
	return base64.StdEncoding.EncodeToString(sig), nil
}

// Verify confirms that the signature embedded in the event matches the payload
// once canonicalized with the provided public key.
func Verify(evt chain.Event, publicKey []byte) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return errors.New("ed25519 public key required")
	}
	if evt.Signature == "" {
		return ErrInvalidSignature
	}
	sig, err := base64.StdEncoding.DecodeString(evt.Signature)
	if err != nil {
		return ErrInvalidSignature
	}
	msg, err := CanonicalEventMessage(evt)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, msg, sig) {
		return ErrInvalidSignature
	}
	return nil
}

// CanonicalEventMessage renders the event payload into a deterministic JSON
// representation that can be signed or verified.
func CanonicalEventMessage(evt chain.Event) ([]byte, error) {
	buf := &bytes.Buffer{}
	buf.WriteByte('{')
	buf.WriteString(`"device_id":`)
	if err := writeJSONString(buf, evt.DeviceID); err != nil {
		return nil, err
	}
	buf.WriteString(`,"nonce":`)
	buf.WriteString(strconv.FormatUint(evt.Nonce, 10))
	buf.WriteString(`,"payload":`)
	if err := encodeCanonicalValue(buf, evt.Payload); err != nil {
		return nil, err
	}
	buf.WriteString(`,"ts":`)
	ts := evt.TS.UTC().Format(time.RFC3339Nano)
	if err := writeJSONString(buf, ts); err != nil {
		return nil, err
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

func writeJSONString(buf *bytes.Buffer, value string) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	buf.Write(data)
	return nil
}

func encodeCanonicalValue(buf *bytes.Buffer, value interface{}) error {
	if value == nil {
		buf.WriteString("null")
		return nil
	}

	switch v := value.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeJSONString(buf, k); err != nil {
				return err
			}
			buf.WriteByte(':')
			if err := encodeCanonicalValue(buf, v[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
		return nil
	case []interface{}:
		buf.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encodeCanonicalValue(buf, item); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case json.Number:
		buf.WriteString(v.String())
		return nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Errorf("marshal value: %w", err)
		}
		buf.Write(data)
		return nil
	}
}
