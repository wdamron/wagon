// Copyright 2017 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !appengine

package compile

import (
	"unsafe"

	mmap "github.com/edsrzf/mmap-go"
)

const (
	minAllocSize = 1024
	// alignment - instruction caching works better on aligned boundaries.
	allocationAlignment = 128 - 1
)

type mmapBlock struct {
	mem       mmap.MMap
	consumed  uint32
	remaining uint32
}

// MMapAllocator copies instructions into executable memory.
type MMapAllocator struct {
	last   *mmapBlock
	blocks []*mmapBlock
}

// Close frees all pages allocted by the allocator.
func (a *MMapAllocator) Close() error {
	for _, block := range a.blocks {
		if err := block.mem.Unmap(); err != nil {
			return err
		}
	}
	return nil
}

// AllocateExec allocates a block of executable memory with the given code contained.
func (a *MMapAllocator) AllocateExec(asm []byte) (NativeCodeUnit, error) {
	// TODO: Use free pages where possible.
	alloc := minAllocSize
	consumed := uint32(len(asm)+allocationAlignment) & ^uint32(allocationAlignment)
	if int(consumed) > alloc { // not big enough? make minAlloc + aligned len
		alloc += int(consumed)
	}
	m, err := mmap.MapRegion(nil, alloc, mmap.EXEC|mmap.RDWR, mmap.ANON, int64(0))
	if err != nil {
		return nil, err
	}
	a.last = &mmapBlock{
		mem:       m,
		consumed:  consumed,
		remaining: uint32(alloc) - consumed,
	}
	a.blocks = append(a.blocks, a.last)
	copy(m, asm)

	out := asmBlock{
		mem: unsafe.Pointer(&m),
	}
	return &out, nil
}
