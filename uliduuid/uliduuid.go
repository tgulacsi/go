// Copyright 2026 Tamás Gulácsi
//
// SPDX-License-Identifier: GPL-3.0

package uliduuid

import (
	"encoding"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/oklog/ulid/v2"
)

type UUID struct {
	uuid.UUID
}

// NewUUID returns a new UUIDv7, with the last counterBits bits zeroed (max 8),
// which may be incremeneted with Inc().
func NewUUID(counterBits int) UUID {
	if counterBits > 8 {
		panic("counterBits must be no more than 8")
	}
	u := UUID{uuid.Must(uuid.NewV7())}
	// 3: 11111000
	// 1<<3 = 00001000
	// (1<<3)-1 = 00000111
	// ^((1<<3)-1) = 11111000
	u.UUID[15] = u.UUID[15] & ^((1 << counterBits) - 1)
	return u
}

// Inc increments the last byte of the UUID, without overflow checking.
func (u *UUID) Inc() { u.UUID[15]++ }

// GetByte sets one byte
func (u UUID) GetByte(i int) byte { return u.UUID[i] }

// SetByte sets one byte
func (u *UUID) SetByte(i int, b byte) { u.UUID[i] = b }

// BytesTo copy the underlying bytes to the given array.
func (u *UUID) BytesTo(b [16]byte) { copy(b[:], u.UUID[:]) }

// ToUUID converts the UUID to ULID
func (u *UUID) ToULID() ULID {
	var uu ULID
	copy(uu.ULID[:], u.UUID[:])
	return uu
}

type ULID struct {
	ulid.ULID
}

// NewULID returns a new ULID, with the last counterBits bits zeroed (max 8),
// which may be incremeneted with Inc().
func NewULID(counterBits int) ULID {
	if counterBits > 8 {
		panic("counterBits must be no more than 8")
	}
	u := ULID{ulid.Make()}
	// 3: 11111000
	// 1<<3 = 00001000
	// (1<<3)-1 = 00000111
	// ^((1<<3)-1) = 11111000
	u.ULID[15] = u.ULID[15] & ^((1 << counterBits) - 1)
	return u
}

// Inc increments the last byte of the UUID, without overflow checking.
func (u *ULID) Inc() { u.ULID[15]++ }

// GetByte sets one byte
func (u ULID) GetByte(i int) byte { return u.ULID[i] }

// SetByte sets one byte
func (u *ULID) SetByte(i int, b byte) { u.ULID[i] = b }

// BytesTo copy the underlying bytes to the given array.
func (u *ULID) BytesTo(b [16]byte) { copy(b[:], u.ULID[:]) }

// ToUUID converts the ULID to UUID.
func (u *ULID) ToUUID() UUID {
	var uu UUID
	copy(uu.UUID[:], u.ULID[:])
	return uu
}

type ID struct {
	uuid  *UUID
	ulid  *ULID
	other []byte
}

func New(x any) (t ID) {
	switch u := x.(type) {
	case UUID:
		t.uuid = &u
	case *UUID:
		t.uuid = u
	case ULID:
		t.ulid = &u
	case *ULID:
		t.ulid = u
	case string:
		if err := t.UnmarshalText([]byte(u)); err != nil {
			panic(fmt.Errorf("unmarshal %q as ID: %w", x, err))
		}
	case []byte:
		if err := t.UnmarshalText(u); err != nil {
			panic(fmt.Errorf("unmarshal %q as ID: %w", x, err))
		}
	default:
		panic(fmt.Errorf("need UUID/ULID, got %T", x))
	}
	return t
}

func (t ID) UUID() UUID {
	if t.uuid != nil {
		return *t.uuid
	} else if t.ulid != nil {
		return t.ulid.ToUUID()
	}
	var u UUID
	if len(t.other) == 16 {
		copy(u.UUID[:], t.other)
	}
	return u
}

func (t ID) ULID() ULID {
	if t.ulid != nil {
		return *t.ulid
	} else if t.uuid != nil {
		return t.uuid.ToULID()
	}
	var u ULID
	if len(t.other) == 16 {
		copy(u.ULID[:], t.other)
	}
	return u
}

func (t ID) IsULID() bool { return t.ulid != nil }
func (t ID) IsUUID() bool { return t.uuid != nil }
func (t ID) IsZero() bool { return t.uuid == nil && t.ulid == nil && len(t.other) == 0 }

// BytesTo copy the underlying bytes to the given array.
func (t ID) BytesTo(b [16]byte) {
	if t.uuid != nil {
		t.uuid.BytesTo(b)
	} else if t.ulid != nil {
		t.ulid.BytesTo(b)
	} else {
		copy(b[:], t.other)
	}
}
func (t ID) String() string {
	if t.uuid != nil {
		return t.uuid.String()
	} else if t.ulid != nil {
		return t.ulid.String()
	}
	return base64.StdEncoding.EncodeToString(t.other)
}
func (t *ID) Inc() {
	if t.uuid != nil {
		t.uuid.Inc()
	} else if t.ulid != nil {
		t.ulid.Inc()
	} else if len(t.other) != 0 {
		t.other[len(t.other)-1]++
	}
}
func (t ID) GetByte(i int) byte {
	if t.uuid != nil {
		return t.uuid.GetByte(i)
	} else if t.ulid != nil {
		return t.ulid.GetByte(i)
	} else if len(t.other) != 0 {
		return t.other[i]
	}
	return 0
}
func (t *ID) SetByte(i int, b byte) {
	if t.uuid != nil {
		t.uuid.SetByte(i, b)
	} else if t.ulid != nil {
		t.ulid.SetByte(i, b)
	} else if len(t.other) != 0 {
		t.other[i] = b
	}
}

var (
	_ encoding.BinaryMarshaler   = ID{}
	_ encoding.BinaryUnmarshaler = (*ID)(nil)
	_ encoding.TextMarshaler     = ID{}
	_ encoding.TextUnmarshaler   = (*ID)(nil)
)

func (t ID) MarshalText() ([]byte, error) {
	if t.uuid != nil {
		return t.uuid.MarshalText()
	} else if t.ulid != nil {
		return t.ulid.MarshalText()
	}
	return base64.StdEncoding.AppendEncode([]byte(nil), t.other), nil
}
func (t ID) MarshalBinary() ([]byte, error) {
	if t.uuid != nil {
		return t.uuid.MarshalBinary()
	} else if t.ulid != nil {
		return t.ulid.MarshalBinary()
	}
	return t.other, nil
}

func (t *ID) UnmarshalText(p []byte) error {
	if len(p) == ulid.EncodedSize {
		if id, err := ulid.Parse(string(p)); err == nil {
			t.uuid, t.ulid, t.other = nil, &ULID{id}, nil
			return nil
		}
	} else if id, err := uuid.Parse(string(p)); err == nil {
		t.uuid, t.ulid, t.other = &UUID{id}, nil, nil
		return nil
	}
	var err error
	if t.other, err = base64.StdEncoding.AppendDecode(t.other[:0], p); err == nil {
		t.uuid, t.ulid = nil, nil
		return nil
	}
	t.uuid, t.ulid, t.other = nil, nil, p
	return nil
}
func (t *ID) UnmarshalBinary(p []byte) error {
	if len(p) != 16 {
		t.uuid, t.ulid, t.other = nil, nil, p
		return nil
	}
	var u uuid.UUID
	copy(u[:], p)
	if u.Version() <= 7 {
		t.uuid, t.ulid, t.other = &UUID{u}, nil, nil
		return nil
	}
	var d ulid.ULID
	copy(d[:], p)
	if d.Time() <= uint64(time.Now().UnixMilli()) {
		t.uuid, t.ulid, t.other = nil, &ULID{d}, p
		return nil
	}
	t.uuid, t.ulid, t.other = nil, nil, p
	return nil
}
