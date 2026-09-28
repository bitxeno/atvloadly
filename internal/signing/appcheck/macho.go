package appcheck

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/smallstep/pkcs7"
	"howett.net/plist"
)

const (
	fatMagic    = 0xcafebabe
	fatMagic64  = 0xcafebabf
	machMagic   = 0xfeedface
	machMagic64 = 0xfeedfacf

	lcCodeSignature = 0x1d

	csMagicEmbeddedSignature    = 0xfade0cc0
	csMagicEmbeddedEntitlements = 0xfade7171
	csMagicBlobWrapper          = 0xfade0b01

	csSlotEntitlements = 5
	csSlotSignature    = 0x10000
)

// codeSignature is what the checks read from the first Mach-O slice.
type codeSignature struct {
	// entitlements is nil when the signature has no entitlements blob.
	entitlements map[string]any
	// signerSHA256 is "" when there is no CMS signer (unsigned or ad hoc).
	signerSHA256 string
}

// hasMachOMagic mirrors the engine's is_macho_dylib: the first four bytes are
// MH_MAGIC, MH_MAGIC_64 or FAT_MAGIC in either byte order.
func hasMachOMagic(f *zip.File) (bool, error) {
	rc, err := f.Open()
	if err != nil {
		return false, err
	}
	defer func() { _ = rc.Close() }()
	var magic [4]byte
	if _, err := io.ReadFull(rc, magic[:]); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return false, nil
		}
		return false, err
	}
	for _, value := range []uint32{binary.BigEndian.Uint32(magic[:]), binary.LittleEndian.Uint32(magic[:])} {
		switch value {
		case machMagic, machMagic64, fatMagic:
			return true, nil
		}
	}
	return false, nil
}

// stream reads an archive entry forward only while tracking the offset.
type stream struct {
	r   io.Reader
	pos int64
}

func (s *stream) read(n int64) ([]byte, error) {
	buf := make([]byte, n)
	read, err := io.ReadFull(s.r, buf)
	s.pos += int64(read)
	if err != nil {
		return nil, fmt.Errorf("truncated Mach-O at offset %d: %w", s.pos, err)
	}
	return buf, nil
}

func (s *stream) skipTo(offset int64) error {
	if offset < s.pos {
		return fmt.Errorf("Mach-O offset %d precedes already read data", offset)
	}
	skipped, err := io.CopyN(io.Discard, s.r, offset-s.pos)
	s.pos += skipped
	if err != nil {
		return fmt.Errorf("truncated Mach-O at offset %d: %w", s.pos, err)
	}
	return nil
}

// readCodeSignature streams the executable entry up to the code signature of
// its first slice, like the engine reading the entitlements of nth_macho(0).
func readCodeSignature(f *zip.File) (*codeSignature, error) {
	if f.UncompressedSize64 > math.MaxInt64 {
		return nil, errors.New("executable is too large")
	}
	size := int64(f.UncompressedSize64)
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	reader := bufio.NewReader(rc)
	s := &stream{r: reader}

	head, err := reader.Peek(4)
	if err != nil {
		return nil, fmt.Errorf("not a Mach-O file: %w", err)
	}
	sliceOffset, sliceEnd := int64(0), size
	switch magic := binary.BigEndian.Uint32(head); magic {
	case fatMagic, fatMagic64:
		header, err := s.read(8)
		if err != nil {
			return nil, err
		}
		count := binary.BigEndian.Uint32(header[4:])
		if count == 0 || count > maxFatArchs {
			return nil, fmt.Errorf("invalid fat architecture count %d", count)
		}
		var offset, length uint64
		if magic == fatMagic {
			arch, err := s.read(20)
			if err != nil {
				return nil, err
			}
			offset, length = uint64(binary.BigEndian.Uint32(arch[8:])), uint64(binary.BigEndian.Uint32(arch[12:]))
		} else {
			arch, err := s.read(32)
			if err != nil {
				return nil, err
			}
			offset, length = binary.BigEndian.Uint64(arch[8:]), binary.BigEndian.Uint64(arch[16:])
		}
		if offset > uint64(size) || length > uint64(size)-offset {
			return nil, errors.New("first fat slice lies outside the file")
		}
		sliceOffset, sliceEnd = int64(offset), int64(offset+length)
		if err := s.skipTo(sliceOffset); err != nil {
			return nil, err
		}
	}

	header, err := s.read(28)
	if err != nil {
		return nil, err
	}
	switch binary.LittleEndian.Uint32(header) {
	case machMagic:
	case machMagic64:
		if _, err := s.read(4); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("not a little-endian Mach-O file")
	}
	ncmds := binary.LittleEndian.Uint32(header[16:])
	sizeofcmds := int64(binary.LittleEndian.Uint32(header[20:]))
	if sizeofcmds > maxLoadCommandsSize || s.pos+sizeofcmds > sliceEnd {
		return nil, fmt.Errorf("invalid load commands size %d", sizeofcmds)
	}
	commands, err := s.read(sizeofcmds)
	if err != nil {
		return nil, err
	}

	var dataOff, dataSize int64
	found := false
	for i, at := uint32(0), int64(0); i < ncmds && !found; i++ {
		if at+8 > sizeofcmds {
			return nil, errors.New("truncated load commands")
		}
		cmd := binary.LittleEndian.Uint32(commands[at:])
		cmdSize := int64(binary.LittleEndian.Uint32(commands[at+4:]))
		if cmdSize < 8 || at+cmdSize > sizeofcmds {
			return nil, fmt.Errorf("invalid load command size %d", cmdSize)
		}
		if cmd == lcCodeSignature {
			if cmdSize < 16 {
				return nil, errors.New("invalid LC_CODE_SIGNATURE")
			}
			dataOff = int64(binary.LittleEndian.Uint32(commands[at+8:]))
			dataSize = int64(binary.LittleEndian.Uint32(commands[at+12:]))
			found = true
		}
		at += cmdSize
	}
	if !found {
		return &codeSignature{}, nil
	}
	if dataSize > maxCodeSignature {
		return nil, fmt.Errorf("code signature is larger than %d bytes", maxCodeSignature)
	}
	start := sliceOffset + dataOff
	if start+dataSize > sliceEnd {
		return nil, errors.New("code signature lies outside the slice")
	}
	if err := s.skipTo(start); err != nil {
		return nil, err
	}
	data, err := s.read(dataSize)
	if err != nil {
		return nil, err
	}
	return parseSuperBlob(data)
}

// parseSuperBlob reads the entitlements and CMS blobs of an embedded signature.
func parseSuperBlob(data []byte) (*codeSignature, error) {
	if len(data) < 12 || binary.BigEndian.Uint32(data) != csMagicEmbeddedSignature {
		return nil, errors.New("invalid embedded signature magic")
	}
	length := uint64(binary.BigEndian.Uint32(data[4:]))
	count := uint64(binary.BigEndian.Uint32(data[8:]))
	if length < 12 || length > uint64(len(data)) || 12+count*8 > length {
		return nil, errors.New("invalid embedded signature length")
	}
	data = data[:length]

	signature := &codeSignature{}
	seen := map[uint32]bool{}
	for i := uint64(0); i < count; i++ {
		slot := binary.BigEndian.Uint32(data[12+i*8:])
		offset := uint64(binary.BigEndian.Uint32(data[16+i*8:]))
		if slot != csSlotEntitlements && slot != csSlotSignature {
			continue
		}
		if seen[slot] {
			return nil, fmt.Errorf("duplicate code signature slot %#x", slot)
		}
		seen[slot] = true
		magic, payload, err := subBlob(data, offset)
		if err != nil {
			return nil, err
		}
		switch slot {
		case csSlotEntitlements:
			if magic != csMagicEmbeddedEntitlements {
				return nil, fmt.Errorf("invalid entitlements blob magic %#x", magic)
			}
			entitlements := map[string]any{}
			if _, err := plist.Unmarshal(payload, &entitlements); err != nil {
				return nil, fmt.Errorf("invalid entitlements: %w", err)
			}
			if entitlements == nil {
				entitlements = map[string]any{}
			}
			signature.entitlements = entitlements
		case csSlotSignature:
			if magic != csMagicBlobWrapper {
				return nil, fmt.Errorf("invalid CMS blob magic %#x", magic)
			}
			signature.signerSHA256 = cmsSignerSHA256(payload)
		}
	}
	return signature, nil
}

func subBlob(data []byte, offset uint64) (magic uint32, payload []byte, err error) {
	if offset+8 > uint64(len(data)) {
		return 0, nil, errors.New("code signature blob lies outside the signature")
	}
	magic = binary.BigEndian.Uint32(data[offset:])
	length := uint64(binary.BigEndian.Uint32(data[offset+4:]))
	if length < 8 || offset+length > uint64(len(data)) {
		return 0, nil, errors.New("invalid code signature blob length")
	}
	return magic, data[offset+8 : offset+length], nil
}

// cmsSignerSHA256 returns the hex SHA-256 of the certificate of the single
// CMS signer, or "" when the blob is empty (ad hoc signature) or its signer
// cannot be identified. An unidentified signer never matches an identity.
func cmsSignerSHA256(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	p7, err := pkcs7.Parse(payload)
	if err != nil {
		return ""
	}
	signer := p7.GetOnlySigner()
	if signer == nil {
		return ""
	}
	sum := sha256.Sum256(signer.Raw)
	return hex.EncodeToString(sum[:])
}
