// Package ids creates sortable, cryptographically random prefixed ULIDs.
package ids

import (
	"crypto/rand"
	"fmt"
	"time"
)

const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

func New(prefix string) (string, error) {
	var raw [16]byte
	millis := uint64(time.Now().UTC().UnixMilli())
	if millis >= 1<<48 {
		return "", fmt.Errorf("ULID timestamp is out of range")
	}
	raw[0] = byte(millis >> 40)
	raw[1] = byte(millis >> 32)
	raw[2] = byte(millis >> 24)
	raw[3] = byte(millis >> 16)
	raw[4] = byte(millis >> 8)
	raw[5] = byte(millis)
	if _, err := rand.Read(raw[6:]); err != nil {
		return "", fmt.Errorf("generate random ULID: %w", err)
	}
	var encoded [26]byte
	for i := range encoded {
		var value byte
		for bit := 0; bit < 5; bit++ {
			position := i*5 + bit - 2 // ULID uses two zero padding bits at the front.
			value <<= 1
			if position >= 0 && position < 128 {
				value |= (raw[position/8] >> (7 - uint(position%8))) & 1
			}
		}
		encoded[i] = alphabet[value]
	}
	return prefix + string(encoded[:]), nil
}
