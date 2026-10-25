package bom

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/bitxeno/atvloadly/pkg/bom/pkg/helper"
	"github.com/bitxeno/atvloadly/pkg/bom/pkg/reader"
)

type BomParser interface {
	// parse bom headers
	Parse() error

	// read all block names
	BlockNames() []string

	// read named block
	ReadBlock(name string) (io.Reader, error)

	// read named tree block
	ReadTree(name string, entry func(k io.Reader, d io.Reader) error) error
}

type bom struct {
	r io.ReadSeeker

	header     *Header
	blockTable *BlockTable
	vars       []Var
}

var _ BomParser = (*bom)(nil)

var (
	// ErrBlockLengthZero = errors.New("block length is zero")
	ErrBlockNotFound = errors.New("block not found")
	ErrNameNotMatch  = errors.New("name not match")
)

func New(r io.ReadSeeker) BomParser {
	return &bom{r: r}
}

func (b *bom) Parse() error {

	f := b.r

	header := &Header{}
	err := binary.Read(f, binary.BigEndian, header)
	if err != nil {
		log.Printf("read header error: %v", err)
		return err
	}
	if HeaderMagic != header.Magic.String() {
		err := fmt.Errorf("header magic not match header.Magic[8] = '%s'", header.Magic.String())
		log.Print(err)
		return err
	}
	b.header = header

	// blockTable
	f.Seek(int64(b.header.IndexOffset), 0)
	blockTable := &BlockTable{}
	// read table block length
	binary.Read(f, binary.BigEndian, &blockTable.NumberOfBlockTablePointers)
	// read table block pointers
	blockTable.BlockPointers = make([]*Pointer, blockTable.NumberOfBlockTablePointers)
	for i := 0; i < int(blockTable.NumberOfBlockTablePointers); i++ {
		p := &Pointer{}
		if err := binary.Read(f, binary.BigEndian, p); err != nil {
			return err
		}
		// First entry must always be a null entry
		if i == 0 && (p.Address != 0 || p.Length != 0) {
			return errors.New("first entry not be a null entry")
		}
		blockTable.BlockPointers[i] = p
	}
	b.blockTable = blockTable

	// read vars
	f.Seek(int64(header.VarsOffset), 0)
	vars := &Vars{}
	binary.Read(f, binary.BigEndian, &vars.Count)
	vars.List = make([]Var, vars.Count)
	for i := 0; i < int(vars.Count); i++ {
		v := Var{}
		binary.Read(f, binary.BigEndian, &v.Index)
		binary.Read(f, binary.BigEndian, &v.Length)

		// parse name
		name, err := helper.ReadString(f, int(v.Length))
		if err != nil {
			log.Printf("read var name error: %v", err)
			return err
		}
		v.Name = name

		vars.List[i] = v
	}
	b.vars = vars.List

	return nil
}

// Block: get block with name
func (b *bom) ReadBlock(name string) (io.Reader, error) {
	for _, v := range b.vars {

		if v.Name != name {
			continue
		}

		b, err := b.blockReader(v.Index)
		if err != nil {
			return nil, err
		}
		return b, nil
	}
	return nil, ErrNameNotMatch
}

func (b *bom) BlockNames() []string {
	names := make([]string, len(b.vars))
	for i, v := range b.vars {
		names[i] = v.Name
	}
	return names
}

func (b *bom) blockReader(index uint32) (io.Reader, error) {
	if index >= uint32(len(b.blockTable.BlockPointers)) {
		return nil, ErrBlockNotFound
	}
	p := b.blockTable.BlockPointers[index]
	return reader.New(b.r, int64(p.Address), int64(p.Length)), nil
}

// Block: get tree block with name
func (b *bom) ReadTree(name string, loop func(k io.Reader, d io.Reader) error) error {
	for _, v := range b.vars {

		if v.Name != name {
			continue
		}

		entryBuf, err := b.blockReader(v.Index)
		if err != nil {
			return err
		}
		entry := TreeEntry{}
		if err := binary.Read(entryBuf, binary.BigEndian, &entry); err != nil {
			return err
		}

		tree, buf, err := b.readTree(entry.Index)
		if err != nil {
			return err
		}
		for tree.IsLeaf == 0 {
			pi := TreeIndex{}
			if err := binary.Read(buf, binary.BigEndian, &pi); err != nil {
				return err
			}
			tree, buf, err = b.readTree(pi.ValueIndex)
			if err != nil {
				return err
			}
		}

		for {
			tree.List = make([]TreeIndex, tree.Count)
			for i := uint16(0); i < tree.Count; i++ {
				pi := TreeIndex{}
				binary.Read(buf, binary.BigEndian, &pi)
				tree.List[i] = pi

				// get key and data
				kbuf, err := b.blockReader(pi.KeyIndex)
				if err != nil {
					// in case of BITMAPKEYS, i don't know why not found, temporary handle
					if err == ErrBlockNotFound {
						p := make([]byte, 4)
						binary.BigEndian.PutUint32(p, pi.KeyIndex)
						kbuf = bytes.NewBuffer(p)
					} else {
						return err
					}
				}
				vbuf, err := b.blockReader(pi.ValueIndex)
				if err != nil {
					// return err
				}
				// loop callback entry
				if err := loop(kbuf, vbuf); err != nil {
					return err
				}
			}

			if tree.Forward == 0 {
				return nil
			}

			tree, buf, err = b.readTree(tree.Forward)
			if err != nil {
				return err
			}
		}
	}

	return ErrNameNotMatch
}

func (b *bom) readTree(index uint32) (*Tree, io.Reader, error) {
	buf, err := b.blockReader(index)
	if err != nil {
		return nil, nil, err
	}

	tree := &Tree{}
	binary.Read(buf, binary.BigEndian, &tree.IsLeaf)
	binary.Read(buf, binary.BigEndian, &tree.Count)
	binary.Read(buf, binary.BigEndian, &tree.Forward)
	binary.Read(buf, binary.BigEndian, &tree.Backward)
	return tree, buf, nil
}
