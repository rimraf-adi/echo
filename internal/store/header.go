package store

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

const (
	// HeaderSize is the fixed size of the binary object header in bytes
	HeaderSize = 48

	// CurrentVersion is the current object format version
	CurrentVersion uint8 = 0x01
)

var (
	// MagicEcho is the 4-byte magic signature "ECHO"
	MagicEcho = [4]byte{'E', 'C', 'H', 'O'}

	ErrInvalidHeaderSize = errors.New("data too short for header")
	ErrInvalidMagic      = errors.New("invalid magic number in object header")
	ErrUnsupportedVersion = errors.New("unsupported object header version")
	ErrInvalidObjectType = errors.New("unknown object type in header")
)

type ObjectType uint8

const (
	ObjectTypeBlob ObjectType = 0x01
	ObjectTypeTree ObjectType = 0x02
)

func (t ObjectType) String() string {
	switch t {
	case ObjectTypeBlob:
		return "blob"
	case ObjectTypeTree:
		return "tree"
	default:
		return fmt.Sprintf("unknown(0x%02x)", uint8(t))
	}
}

const (
	FlagCompressed uint8 = 1 << 0
)

// Header represents the 48-byte fixed binary header for Echo objects on disk
type Header struct {
	Magic      [4]byte
	Version    uint8
	ObjType    ObjectType
	Flags      uint8
	Reserved   uint8
	OrigSize   uint32
	StoredSize uint32
	Hash       [32]byte
}

// IsCompressed returns true if the compressed flag is set
func (h *Header) IsCompressed() bool {
	return (h.Flags & FlagCompressed) != 0
}

// HashHex returns the SHA-256 hash as a lower-case hexadecimal string
func (h *Header) HashHex() string {
	return hex.EncodeToString(h.Hash[:])
}

// SetHashHex parses a 64-character hex string into the 32-byte Hash field
func (h *Header) SetHashHex(hashHex string) error {
	b, err := hex.DecodeString(hashHex)
	if err != nil {
		return err
	}
	if len(b) != 32 {
		return fmt.Errorf("expected 32 bytes hash, got %d", len(b))
	}
	copy(h.Hash[:], b)
	return nil
}

// EncodeHeader serializes the Header into a 48-byte slice
func EncodeHeader(h Header) []byte {
	buf := make([]byte, HeaderSize)
	copy(buf[0:4], h.Magic[:])
	buf[4] = h.Version
	buf[5] = byte(h.ObjType)
	buf[6] = h.Flags
	buf[7] = h.Reserved
	binary.BigEndian.PutUint32(buf[8:12], h.OrigSize)
	binary.BigEndian.PutUint32(buf[12:16], h.StoredSize)
	copy(buf[16:48], h.Hash[:])
	return buf
}

// DecodeHeader deserializes and validates a 48-byte slice into a Header
func DecodeHeader(data []byte) (Header, error) {
	if len(data) < HeaderSize {
		return Header{}, ErrInvalidHeaderSize
	}

	var h Header
	copy(h.Magic[:], data[0:4])
	if h.Magic != MagicEcho {
		return Header{}, ErrInvalidMagic
	}

	h.Version = data[4]
	if h.Version != CurrentVersion {
		return Header{}, fmt.Errorf("%w: %d", ErrUnsupportedVersion, h.Version)
	}

	h.ObjType = ObjectType(data[5])
	if h.ObjType != ObjectTypeBlob && h.ObjType != ObjectTypeTree {
		return Header{}, fmt.Errorf("%w: 0x%02x", ErrInvalidObjectType, data[5])
	}

	h.Flags = data[6]
	h.Reserved = data[7]
	h.OrigSize = binary.BigEndian.Uint32(data[8:12])
	h.StoredSize = binary.BigEndian.Uint32(data[12:16])
	copy(h.Hash[:], data[16:48])

	return h, nil
}
