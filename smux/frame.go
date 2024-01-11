package smux

import (
	"encoding/binary"
	"fmt"
	"time"
	"crypto/rand"
)

const ( // cmds
	// protocol version 1:
	cmdSYN byte = iota // stream open
	cmdFIN             // stream close, a.k.a EOF mark
	cmdPSH             // data push
	cmdNOP             // no operation

	// protocol version 2 extra commands
	// notify bytes consumed by remote peer-end
	cmdUPD
)

const (
	// data size of cmdUPD, format:
	// |4B data consumed(ACK)| 4B window size(WINDOW) |
	szCmdUPD = 8
)

const (
	// initial peer window guess, a slow-start
	initialPeerWindow = 262144
)

const (
	sizeOfVer    = 1
	sizeOfCmd    = 1
	sizeOfLength = 2
	sizeOfSid    = 4
	headerSize   = sizeOfVer + sizeOfCmd + sizeOfSid + sizeOfLength
	encryptedHeaderSize = 20
)

// Frame defines a packet from or to be multiplexed into a single connection
type Frame struct {
	ver  byte
	cmd  byte
	sid  uint32
	data []byte
}


func newFrame(version byte, cmd byte, sid uint32) Frame {
	return Frame{ver: version, cmd: cmd, sid: sid}
}

type rawHeader [headerSize]byte

func (h rawHeader) Version() byte {
	return h[0]
}

func (h rawHeader) Cmd() byte {
	return h[1]
}

func (h rawHeader) Length() uint16 {
	return binary.LittleEndian.Uint16(h[2:])
}

func (h rawHeader) StreamID() uint32 {
	return binary.LittleEndian.Uint32(h[4:])
}

func (h rawHeader) String() string {
	return fmt.Sprintf("Version:%d Cmd:%d StreamID:%d Length:%d",
		h.Version(), h.Cmd(), h.StreamID(), h.Length())
}

type updHeader [szCmdUPD]byte

func (h updHeader) Consumed() uint32 {
	return binary.LittleEndian.Uint32(h[:])
}
func (h updHeader) Window() uint32 {
	return binary.LittleEndian.Uint32(h[4:])
}


type encryptedHeader struct {
	eb 		[encryptedHeaderSize]byte
	pkr 	*Keyring
}

func NewEncryptedHeader(k *Keyring) *encryptedHeader{
	e := &encryptedHeader{
		pkr: k,
	}
	return e
}

func (e *encryptedHeader) Mask() {
	// Mask Timestamp
	copy(e.eb[6:10], XORBytes(e.eb[6:10], e.pkr.Extract(SHA256(e.eb[:6]), "timestamp")))

	// Mask version
	copy(e.eb[10:11], XORBytes(e.eb[10:11], e.pkr.Extract(SHA256(e.eb[:6]), "version")))
	
	// Mask CMD
	copy(e.eb[11:12], XORBytes(e.eb[11:12],  e.pkr.Extract(SHA256(e.eb[:6]), "cmd")))

	// Mask SID
	copy(e.eb[12:16], XORBytes(e.eb[12:16], e.pkr.Extract(SHA256(e.eb[:6]), "sid")))

	// Mask LEN
	copy(e.eb[16:18], XORBytes(e.eb[16:18], e.pkr.Extract(SHA256(e.eb[:6]), "len")))

	// Mask CHKSUM
	copy(e.eb[18:20], XORBytes(e.eb[18:20], e.pkr.Extract(SHA256(e.eb[:6]), "chksum")))
}

func (e *encryptedHeader) Unmask() {
		// Mask Timestamp
		copy(e.eb[6:10], XORBytes(e.eb[6:10], e.pkr.Extract(SHA256(e.eb[:6]), "timestamp")))

		// Mask version
		copy(e.eb[10:11], XORBytes(e.eb[10:11], e.pkr.Extract(SHA256(e.eb[:6]), "version")))
		
		// Mask CMD
		copy(e.eb[11:12], XORBytes(e.eb[11:12],  e.pkr.Extract(SHA256(e.eb[:6]), "cmd")))
	
		// Mask SID
		copy(e.eb[12:16], XORBytes(e.eb[12:16], e.pkr.Extract(SHA256(e.eb[:6]), "sid")))
	
		// Mask LEN
		copy(e.eb[16:18], XORBytes(e.eb[16:18], e.pkr.Extract(SHA256(e.eb[:6]), "len")))
	
		// Mask CHKSUM
		copy(e.eb[18:20], XORBytes(e.eb[18:20], e.pkr.Extract(SHA256(e.eb[:6]), "chksum")))
}

func (e *encryptedHeader) SetEncryptedHeader(cmd byte, sid uint32, cipherLen uint16){
	// Set IV
	rand.Read(e.eb[:6])

	// Set Timestamp
	binary.LittleEndian.PutUint32(e.eb[6:10], uint32(time.Now().Unix()))

	// Set Version
	e.eb[10]=0x01

	// Set CMD
	e.eb[11]=cmd

	// Set SessionID
	binary.LittleEndian.PutUint32(e.eb[12:16], sid)

	// Set Data length
	binary.LittleEndian.PutUint16(e.eb[16:18],cipherLen)
	
	// Set Checksum
	copy(e.eb[18:], SHA256(e.eb[:18])[:2])
}

func (e *encryptedHeader) IV() []byte{
	return e.eb[:6]
}

func (e *encryptedHeader) Version() byte{
	return e.eb[6]
}

func (e *encryptedHeader) Timestamp() []byte{
	return e.eb[6:10]
}

func (e *encryptedHeader) StreamID() uint32{
	return binary.LittleEndian.Uint32(e.eb[12:16])
}

func (e *encryptedHeader) Length() uint16{
	return binary.LittleEndian.Uint16(e.eb[16:18])
}

func (e *encryptedHeader) CMD() byte{
	return e.eb[11]
}

func (e *encryptedHeader) Chksum() []byte{
	return e.eb[18:20]
}