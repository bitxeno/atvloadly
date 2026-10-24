// Base on:
// https://github.com/hogliux/bomutils/blob/master/src/bom.h
// https://blog.timac.org/2018/1018-reverse-engineering-the-car-file-format/

package bom

import (
	"github.com/bitxeno/atvloadly/pkg/bom/pkg/helper"
)

const HeaderMagic = "BOMStore"

type Header struct {
	// char magic[8];
	Magic helper.String8 // Always "BOMStore"
	// uint32_t version;         // Always 1
	Version uint32
	// uint32_t numberOfBlocks;  // Number of non-null entries in BOMBlockTable
	NumberOfBlocks uint32
	// uint32_t indexOffset;     // Offset to index table
	IndexOffset uint32
	// uint32_t indexLength;     // Length of index table, indexOffset + indexLength = file_length
	IndexLength uint32
	// uint32_t varsOffset;
	VarsOffset uint32
	// uint32_t varsLength;
	VarsLength uint32
	_          [480]byte
}

type Pointer struct {
	// uint32_t address;
	Address uint32
	// uint32_t length;
	Length uint32
}

type BlockTable struct {
	// uint32_t numberOfBlockTablePointers;  // Not all of them will be occupied. See header for number of non-null blocks
	NumberOfBlockTablePointers uint32
	// BOMPointer blockPointers[];           // First entry must always be a null entry
	BlockPointers []*Pointer
}

type Vars struct {
	// uint32_t count;
	Count uint32
	// BOMVar first[];
	List []Var
}

type Var struct {
	// uint32_t index;
	Index uint32 // BlockTable index
	// uint8_t length;
	Length uint8 // var name length
	// char name[];
	Name string // var name
}

// tag: 'tree'
type TreeEntry struct {
	// uint32_t tag;
	Tag helper.String4
	// uint32_t version;     // Always 1
	Version uint32
	// uint32_t child;       // Index for BOMPaths
	Index uint32
	// uint32_t blockSize;   // Always 4096
	BlockSize uint32
	// uint32_t pathCount;   // Total number of paths in all leaves combined
	PathCount uint32
	// uint8_t unknown3;
	Unknown3 uint8
}

type TreeIndex struct {
	// uint32_t index0; /* for leaf: points to BOMPathInfo1, for branch points to BOMPaths */
	ValueIndex uint32
	// uint32_t index1; /* always points to BOMFile */
	KeyIndex uint32
}

type Tree struct {
	IsLeaf   uint16
	Count    uint16
	Forward  uint32
	Backward uint32
	List     []TreeIndex
}
