package iplib

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Magic is the four-byte signature at the start of every .ipdb file.
const Magic = "IPDB"

// fileVersion is the current binary format version.
const fileVersion uint16 = 1

// headerSize is the fixed size of the file header in bytes.
const headerSize = 64

// indexEntry4Size is the size of one IPv4 index entry:
//
//	StartIP(4) + EndIP(4) + DataOffset(4) + DataLength(2) = 14 bytes
const indexEntry4Size = 14

// indexEntry6Size is the size of one IPv6 index entry:
//
//	StartIP(16) + EndIP(16) + DataOffset(4) + DataLength(2) = 38 bytes
const indexEntry6Size = 38

// Flags stored in the header Flags byte.
const (
	flagHasIPv4 uint8 = 1 << 0
	flagHasIPv6 uint8 = 1 << 1
)

// dbHeader represents the 64-byte file header.
//
// Byte layout:
//
//	[0 :4 ]  Magic   "IPDB"
//	[4 :6 ]  Version uint16 LE
//	[6    ]  Flags   uint8
//	[7    ]  reserved
//	[8 :16]  BuildTime int64 LE (Unix seconds)
//	[16:20]  IPv4Count uint32 LE
//	[20:24]  IPv6Count uint32 LE
//	[24:28]  DataSize  uint32 LE  (total bytes in data section)
//	[28:32]  MetaOffset uint32 LE (offset of Meta record in data section)
//	[32:36]  MetaLength uint32 LE (length of Meta record in data section)
//	[36:64]  reserved (zeroed)
type dbHeader struct {
	Magic      [4]byte
	Version    uint16
	Flags      uint8
	BuildTime  int64
	IPv4Count  uint32
	IPv6Count  uint32
	DataSize   uint32
	MetaOffset uint32
	MetaLength uint32
}

var le = binary.LittleEndian

// encodeHeader serialises h into a 64-byte slice.
func encodeHeader(h *dbHeader) []byte {
	buf := make([]byte, headerSize)
	copy(buf[0:4], h.Magic[:])
	le.PutUint16(buf[4:6], h.Version)
	buf[6] = h.Flags
	// buf[7] reserved
	le.PutUint64(buf[8:16], uint64(h.BuildTime))
	le.PutUint32(buf[16:20], h.IPv4Count)
	le.PutUint32(buf[20:24], h.IPv6Count)
	le.PutUint32(buf[24:28], h.DataSize)
	le.PutUint32(buf[28:32], h.MetaOffset)
	le.PutUint32(buf[32:36], h.MetaLength)
	// [36:64] zeroed by make
	return buf
}

// decodeHeader parses the first headerSize bytes of buf.
func decodeHeader(buf []byte) (*dbHeader, error) {
	if len(buf) < headerSize {
		return nil, errors.New("iplib: file too small to contain a valid header")
	}
	h := &dbHeader{}
	copy(h.Magic[:], buf[0:4])
	if string(h.Magic[:]) != Magic {
		return nil, fmt.Errorf("iplib: invalid magic %q", string(h.Magic[:]))
	}
	h.Version = le.Uint16(buf[4:6])
	if h.Version != fileVersion {
		return nil, fmt.Errorf("iplib: unsupported file version %d (want %d)", h.Version, fileVersion)
	}
	h.Flags = buf[6]
	h.BuildTime = int64(le.Uint64(buf[8:16]))
	h.IPv4Count = le.Uint32(buf[16:20])
	h.IPv6Count = le.Uint32(buf[20:24])
	h.DataSize = le.Uint32(buf[24:28])
	h.MetaOffset = le.Uint32(buf[28:32])
	h.MetaLength = le.Uint32(buf[32:36])
	return h, nil
}

// encodeIndex4 serialises one IPv4 index entry into exactly indexEntry4Size
// bytes, appended to dst.
func encodeIndex4(dst []byte, startIP, endIP uint32, dataOffset uint32, dataLen uint16) []byte {
	var b [indexEntry4Size]byte
	le.PutUint32(b[0:4], startIP)
	le.PutUint32(b[4:8], endIP)
	le.PutUint32(b[8:12], dataOffset)
	le.PutUint16(b[12:14], dataLen)
	return append(dst, b[:]...)
}

// encodeIndex6 serialises one IPv6 index entry into exactly indexEntry6Size
// bytes, appended to dst.
func encodeIndex6(dst []byte, startIP, endIP [16]byte, dataOffset uint32, dataLen uint16) []byte {
	var b [indexEntry6Size]byte
	copy(b[0:16], startIP[:])
	copy(b[16:32], endIP[:])
	le.PutUint32(b[32:36], dataOffset)
	le.PutUint16(b[36:38], dataLen)
	return append(dst, b[:]...)
}

// ip4ToUint32 converts a 4-byte IPv4 address (big-endian) to a uint32 that
// preserves the natural address ordering used by binary search.
func ip4ToUint32(ip []byte) uint32 {
	return uint32(ip[0])<<24 | uint32(ip[1])<<16 | uint32(ip[2])<<8 | uint32(ip[3])
}

// compareIP6 compares two 16-byte IPv6 addresses lexicographically and returns
// -1, 0, or +1. This is equivalent to bytes.Compare but operates on fixed-size
// arrays, avoiding a slice allocation.
func compareIP6(a, b [16]byte) int {
	for i := 0; i < 16; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
