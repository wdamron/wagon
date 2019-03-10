package native

import (
	"unsafe"

	mmap "github.com/edsrzf/mmap-go"
)

const (
	// allocate blocks of 32k.
	minAllocSize = 32 * 1024
	// alignment - instruction caching works better on aligned boundaries.
	allocationAlignment = 2048 - 1
)

type mmapBlock struct {
	mem       mmap.MMap
	consumed  uint32
	remaining uint32
}

// MMapAllocator copies instructions into executable memory.
type MMapAllocator struct {
	last *mmapBlock
}

// AllocateExec allocates a block of executable memory with the given code contained.
func (a *MMapAllocator) AllocateExec(asm []byte) (unsafe.Pointer, error) {
	if a.last != nil && a.last.remaining > uint32(len(asm)) {
		copy(a.last.mem[a.last.consumed:], asm)
		ptr := unsafe.Pointer(&a.last.mem[a.last.consumed])

		alignedConsumption := uint32(len(asm)+allocationAlignment) & ^uint32(allocationAlignment)
		a.last.consumed += alignedConsumption
		a.last.remaining -= alignedConsumption
		return ptr, nil
	}

	// can't use last allocation - make new block.
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
	copy(m[:len(asm)], asm)
	return unsafe.Pointer(&m[0]), nil
}
