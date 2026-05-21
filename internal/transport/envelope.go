package transport

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Envelope represents a message exchanged between the client and server.
type Envelope struct {
	// SessionID is the unique identifier for the connection (UUID).
	SessionID string `json:"session_id"`

	// Seq is the sequence number for ordering packets.
	Seq uint64 `json:"seq"`

	// TargetAddr is used by the client on the first sequence to tell the server where to connect.
	TargetAddr string `json:"target_addr,omitempty"`

	// Payload contains the actual application data.
	Payload []byte `json:"payload,omitempty"`

	// Close implies that the sender is closing its write side of the session.
	Close bool `json:"close,omitempty"`
}

const (
	MagicByte = 0x1F
)

// Encode writes the envelope directly to an io.Writer.
func (e *Envelope) Encode(w io.Writer) error {
	var hdr [1024]byte // magic(1)+sidLen(1)+sid(≤255)+seq(8)+addrLen(1)+addr(≤255)+close(1)+payLen(4) = max 526
	hdr[0] = MagicByte
	hdr[1] = uint8(len(e.SessionID))
	copy(hdr[2:], e.SessionID)
	offset := 2 + len(e.SessionID)

	binary.BigEndian.PutUint64(hdr[offset:], e.Seq)
	offset += 8

	hdr[offset] = uint8(len(e.TargetAddr))
	offset++
	copy(hdr[offset:], e.TargetAddr)
	offset += len(e.TargetAddr)

	if e.Close {
		hdr[offset] = 1
	} else {
		hdr[offset] = 0
	}
	offset++

	binary.BigEndian.PutUint32(hdr[offset:], uint32(len(e.Payload)))
	offset += 4

	if _, err := w.Write(hdr[:offset]); err != nil {
		return err
	}
	if len(e.Payload) > 0 {
		_, err := w.Write(e.Payload)
		return err
	}
	return nil
}

// Decode reads an envelope from an io.Reader.
func (e *Envelope) Decode(r io.Reader) error {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return err
	}
	if hdr[0] != MagicByte {
		return fmt.Errorf("invalid magic byte: 0x%X", hdr[0])
	}
	sidLen := int(hdr[1])
	sidBuf := make([]byte, sidLen)
	if _, err := io.ReadFull(r, sidBuf); err != nil {
		return err
	}
	e.SessionID = string(sidBuf)

	var seqBuf [8]byte
	if _, err := io.ReadFull(r, seqBuf[:]); err != nil {
		return err
	}
	e.Seq = binary.BigEndian.Uint64(seqBuf[:])

	var addrLenBuf [1]byte
	if _, err := io.ReadFull(r, addrLenBuf[:]); err != nil {
		return err
	}
	addrLen := int(addrLenBuf[0])
	addrBuf := make([]byte, addrLen)
	if _, err := io.ReadFull(r, addrBuf); err != nil {
		return err
	}
	e.TargetAddr = string(addrBuf)

	var closeBuf [1]byte
	if _, err := io.ReadFull(r, closeBuf[:]); err != nil {
		return err
	}
	e.Close = closeBuf[0] == 1

	var payLenBuf [4]byte
	if _, err := io.ReadFull(r, payLenBuf[:]); err != nil {
		return err
	}
	payLen := binary.BigEndian.Uint32(payLenBuf[:])
	if payLen > 10*1024*1024 { // Sanity check: 10MB max packet
		return fmt.Errorf("packet too large: %d", payLen)
	}
	if payLen > 0 {
		e.Payload = make([]byte, payLen)
		if _, err := io.ReadFull(r, e.Payload); err != nil {
			return err
		}
	} else {
		e.Payload = nil
	}
	return nil
}
