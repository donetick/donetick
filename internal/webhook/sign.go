package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"time"
)

// Headers Donetick sets on every delivery. A receiver can use the signature
// to tell a real delivery from anyone who learned the webhook URL, and the
// delivery id to drop duplicates.
const (
	HeaderSignature = "X-Donetick-Signature"
	HeaderTimestamp = "X-Donetick-Timestamp"
	HeaderEvent     = "X-Donetick-Event"
	HeaderDelivery  = "X-Donetick-Delivery"

	UserAgent = "Donetick-Webhook/1"
)

// Sign returns the value for HeaderSignature: an HMAC-SHA256 over
// "<unix timestamp>.<body>", in the "t=<ts>,v1=<hex>" form Stripe-style
// receivers already know how to verify.
//
// The timestamp is inside the signed material on purpose: without it a
// captured delivery could be replayed with a fresh timestamp header.
func Sign(secret string, ts time.Time, body []byte) string {
	unix := strconv.FormatInt(ts.Unix(), 10)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unix))
	mac.Write([]byte("."))
	mac.Write(body)

	return fmt.Sprintf("t=%s,v1=%s", unix, hex.EncodeToString(mac.Sum(nil)))
}

// NewDeliveryID returns a random id for a single delivery attempt.
func NewDeliveryID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// A non-unique id only costs the receiver its duplicate
		// detection, so fall back rather than drop the delivery.
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b[:])
}

func newBodyReader(body []byte) io.Reader {
	return bytes.NewReader(body)
}
