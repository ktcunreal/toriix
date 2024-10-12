package smux

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"github.com/djherbis/buffer"
	"github.com/djherbis/nio/v3"
	"io"
	"sync"
	"time"
)

type Keyring struct {
	p string
}

func NewKeyring(s string) *Keyring {
	Keyring := &Keyring{
		p: s,
	}
	return Keyring
}

func (k *Keyring) Extract(iv []byte, s string) []byte {
	tail := []byte(k.p + s)
	b := make([]byte, len(iv)+len(tail))
	copy(b[:len(iv)], iv)
	copy(b[len(iv):], tail)
	return SHA256(b)
}

func SHA256(b []byte) []byte {
	s := sha256.Sum256(b)
	return s[:]
}

func ChunkBytes(b []byte, chunkSize int) [][]byte {
	if chunkSize <= 0 {
		return nil
	}
	var chunks [][]byte
	for i := 0; i < len(b); i += chunkSize {
		end := i + chunkSize
		if end > len(b) {
			end = len(b)
		}
		chunks = append(chunks, b[i:end])
	}
	return chunks
}

// func randomASCIIChar() byte {
// 	rand.Seed(time.Now().UnixNano())
// 	randomInt := rand.Intn(95) + 32
// 	return byte(randomInt)
// }

func XORBytes(a, b []byte) []byte {
	buf := make([]byte, len(a))
	for i, _ := range a {
		buf[i] = a[i] ^ b[i]
	}
	return buf
}

func increment(b *[24]byte) {
	for i := range b {
		b[i]++
		if b[i] != 0 {
			return
		}
	}
}

type encryptedHeader struct {
	eb  [encryptedHeaderSize]byte
	pkr *Keyring
}

func NewEncryptedHeader(k *Keyring) *encryptedHeader {
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
	copy(e.eb[11:12], XORBytes(e.eb[11:12], e.pkr.Extract(SHA256(e.eb[:6]), "cmd")))

	// Mask SID
	copy(e.eb[12:16], XORBytes(e.eb[12:16], e.pkr.Extract(SHA256(e.eb[:6]), "sid")))

	// Mask LEN
	copy(e.eb[16:18], XORBytes(e.eb[16:18], e.pkr.Extract(SHA256(e.eb[:6]), "len")))

	// Mask CHKSUM
	copy(e.eb[18:20], XORBytes(e.eb[18:20], e.pkr.Extract(SHA256(e.eb[:6]), "chksum")))
}

func (e *encryptedHeader) SetEncryptedHeader(cmd byte, sid uint32, cipherLen uint16) {
	// Set IV
	rand.Read(e.eb[:6])

	// Set Timestamp
	binary.LittleEndian.PutUint32(e.eb[6:10], uint32(time.Now().Unix()))

	// Set Version
	e.eb[10] = 0x01

	// Set CMD
	e.eb[11] = cmd

	// Set SessionID
	binary.LittleEndian.PutUint32(e.eb[12:16], sid)

	// Set Data length
	binary.LittleEndian.PutUint16(e.eb[16:18], cipherLen)

	// Set Checksum
	copy(e.eb[18:], e.pkr.Extract(SHA256(e.eb[:6]), "chksum")[:2])
	copy(e.eb[18:], SHA256(e.eb[:])[:2])
}

func (e *encryptedHeader) ValidEncryptedHeader() bool {
	// Validate timestamp
	if Abs(int(time.Now().Unix())-int(binary.LittleEndian.Uint32(e.Timestamp()))) > 180 {
		return false
	}

	// Validate Checksum
	headerChksum := e.Chksum()

	copy(e.eb[18:], e.pkr.Extract(SHA256(e.eb[:6]), "chksum")[:2])
	chksum := SHA256(e.eb[:])[:2]

	if !bytes.Equal(headerChksum, chksum) {
		return false
	}
	return true
}

func (e *encryptedHeader) IV() []byte {
	return e.eb[:6]
}

func (e *encryptedHeader) Version() byte {
	return e.eb[6]
}

func (e *encryptedHeader) Timestamp() []byte {
	return e.eb[6:10]
}

func (e *encryptedHeader) StreamID() uint32 {
	return binary.LittleEndian.Uint32(e.eb[12:16])
}

func (e *encryptedHeader) Length() uint16 {
	return binary.LittleEndian.Uint16(e.eb[16:18])
}

func (e *encryptedHeader) CMD() byte {
	return e.eb[11]
}

func (e *encryptedHeader) Chksum() []byte {
	buf := make([]byte, 2)
	copy(buf, e.eb[18:20])
	return buf
}

func Abs(i int) int {
	if i < 0 {
		return -i
	}
	return i
}

func NPipe(src, dst io.ReadWriteCloser) {
	go func() {
		defer src.Close()
		if _, err := nio.Copy(dst, src, buffer.New(32*1024)); err != nil {
			return
		}
	}()
	go func() {
		defer dst.Close()
		if _, err := nio.Copy(src, dst, buffer.New(32*1024)); err != nil {
			//log.Println(err)
			return
		}
	}()
}

// Borrowed from kcptun
// Pipe create a general bidirectional pipe between two streams
func KPipe(alice, bob io.ReadWriteCloser, closeWait int) (errA, errB error) {
	var closed sync.Once

	var wg sync.WaitGroup
	wg.Add(2)

	streamCopy := func(dst io.Writer, src io.ReadCloser, err *error) {
		// write error directly to the *pointer
		_, *err = nio.Copy(dst, src, buffer.New(32*1024))
		if closeWait > 0 {
			<-time.After(time.Duration(closeWait) * time.Second)
		}

		// wg.Done() called
		wg.Done()

		// close only once
		closed.Do(func() {
			alice.Close()
			bob.Close()
		})
	}

	// start bidirectional stream copying
	go streamCopy(alice, bob, &errA)
	go streamCopy(bob, alice, &errB)

	// wait for both direction to close
	wg.Wait()

	return
}
