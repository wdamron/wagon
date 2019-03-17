// Copyright 2019 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !appengine amd64

package compile

import (
	"encoding/binary"
	"runtime"
	"testing"
	"unsafe"

	ops "github.com/go-interpreter/wagon/wasm/operators"
	asm "github.com/twitchyliquid64/golang-asm"
	"github.com/twitchyliquid64/golang-asm/obj"
	"github.com/twitchyliquid64/golang-asm/obj/x86"
)

func TestAMD64StackPush(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	b := &AMD64Backend{}
	regs := &dirtyRegs{}
	b.emitPreamble(builder, regs)
	mov := builder.NewProg()
	mov.As = x86.AMOVQ
	mov.From.Type = obj.TYPE_CONST
	mov.From.Offset = 1234
	mov.To.Type = obj.TYPE_REG
	mov.To.Reg = x86.REG_AX
	builder.AddInstruction(mov)
	b.emitWasmStackPush(builder, regs, x86.REG_AX)
	mov = builder.NewProg()
	mov.As = x86.AMOVQ
	mov.From.Type = obj.TYPE_CONST
	mov.From.Offset = 5678
	mov.To.Type = obj.TYPE_REG
	mov.To.Reg = x86.REG_AX
	builder.AddInstruction(mov)
	b.emitWasmStackPush(builder, regs, x86.REG_AX)
	b.emitPostamble(builder, regs)
	out := builder.Assemble()

	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 0, 5)
	fakeLocals := make([]uint64, 0, 0)
	nativeBlock.Invoke(&fakeStack, &fakeLocals)

	if got, want := len(fakeStack), 2; got != want {
		t.Errorf("fakeStack.Len = %d, want %d", got, want)
	}
	if got, want := fakeStack[0], uint64(1234); got != want {
		t.Errorf("fakeStack[0] = %d, want %d", got, want)
	}
	if got, want := fakeStack[1], uint64(5678); got != want {
		t.Errorf("fakeStack[1] = %d, want %d", got, want)
	}
}

func TestAMD64StackPop(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	b := &AMD64Backend{}
	regs := &dirtyRegs{}
	b.emitPreamble(builder, regs)
	b.emitWasmStackLoad(builder, regs, x86.REG_AX)
	b.emitWasmStackLoad(builder, regs, x86.REG_BX)
	b.emitWasmStackPush(builder, regs, x86.REG_AX)
	b.emitPostamble(builder, regs)
	out := builder.Assemble()

	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 2, 5)
	fakeStack[1] = 1337
	fakeLocals := make([]uint64, 0, 0)
	nativeBlock.Invoke(&fakeStack, &fakeLocals)

	if got, want := len(fakeStack), 1; got != want {
		t.Errorf("fakeStack.Len = %d, want %d", got, want)
	}
	if got, want := fakeStack[0], uint64(1337); got != want {
		t.Errorf("fakeStack[0] = %d, want %d", got, want)
	}
}

func TestAMD64LocalsGet(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	b := &AMD64Backend{}
	regs := &dirtyRegs{}
	b.emitPreamble(builder, regs)
	b.emitWasmLocalsLoad(builder, regs, x86.REG_AX, 0)
	b.emitWasmStackPush(builder, regs, x86.REG_AX)
	b.emitWasmLocalsLoad(builder, regs, x86.REG_AX, 1)
	b.emitWasmStackPush(builder, regs, x86.REG_AX)
	b.emitBinaryI64(builder, regs, ops.I64Add)
	b.emitPostamble(builder, regs)
	out := builder.Assemble()

	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 0, 5)
	fakeLocals := make([]uint64, 2, 2)
	fakeLocals[0] = 1335
	fakeLocals[1] = 2
	nativeBlock.Invoke(&fakeStack, &fakeLocals)

	if got, want := len(fakeStack), 1; got != want {
		t.Errorf("fakeStack.Len = %d, want %d", got, want)
	}
	if got, want := fakeStack[0], uint64(1337); got != want {
		t.Errorf("fakeStack[0] = %d, want %d", got, want)
	}
}

func TestAMD64OperationsI64(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.SkipNow()
	}
	testCases := []struct {
		Name   string
		Op     byte
		Args   []uint64
		Result uint64
	}{
		{
			Name:   "add",
			Op:     ops.I64Add,
			Args:   []uint64{12, 3},
			Result: 15,
		},
		{
			Name:   "subtract",
			Op:     ops.I64Sub,
			Args:   []uint64{12, 3},
			Result: 9,
		},
		{
			Name:   "and",
			Op:     ops.I64And,
			Args:   []uint64{15, 3},
			Result: 3,
		},
		{
			Name:   "or",
			Op:     ops.I64Or,
			Args:   []uint64{1, 2},
			Result: 3,
		},
		{
			Name:   "multiply",
			Op:     ops.I64Mul,
			Args:   []uint64{11, 5},
			Result: 55,
		},
	}

	allocator := &MMapAllocator{}
	b := &AMD64Backend{}
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			regs := &dirtyRegs{}
			builder, err := asm.NewBuilder("amd64", 64)
			if err != nil {
				t.Fatal(err)
			}
			b.emitPreamble(builder, regs)

			for _, arg := range tc.Args {
				b.emitPushI64(builder, regs, arg)
			}
			b.emitBinaryI64(builder, regs, tc.Op)
			b.emitPostamble(builder, regs)
			out := builder.Assemble()

			// cmd := exec.Command("ndisasm", "-b64", "-")
			// cmd.Stdin = bytes.NewReader(out)
			// cmd.Stdout = os.Stdout
			// cmd.Run()

			nativeBlock, err := allocator.AllocateExec(out)
			if err != nil {
				t.Fatal(err)
			}

			fakeStack := make([]uint64, 0, 5)
			fakeLocals := make([]uint64, 0, 0)
			nativeBlock.Invoke(&fakeStack, &fakeLocals)

			if got, want := len(fakeStack), 1; got != want {
				t.Fatalf("fakeStack.Len = %d, want %d", got, want)
			}
			if got, want := fakeStack[0], tc.Result; got != want {
				t.Errorf("fakeStack[0] = %d, want %d", got, want)
			}
		})
	}
}

// TestSliceMemoryLayoutAMD64 tests assumptions about the memory layout
// of slices have not changed. These are not specified in the Go
// spec.
// Specifically, we expect the Go compiler lays out slice headers
// like this:
//    0000: pointer to first element
//    0008: uint64 length of the slice
//    0010: uint64 capacity of the slice.
//
// This test should fail if this ever changes. In that case, stack handling
// instructions that are emitted (emitWasmStackLoad/emitWasmStackPush) will
// need to be revised to match the new memory layout.
func TestSliceMemoryLayoutAMD64(t *testing.T) {
	slice := make([]uint64, 2, 5)
	mem := (*[24]byte)(unsafe.Pointer(&slice))
	if got, want := binary.LittleEndian.Uint64(mem[8:16]), uint64(2); got != want {
		t.Errorf("Got len = %d, want %d", got, want)
	}
	if got, want := binary.LittleEndian.Uint64(mem[16:24]), uint64(5); got != want {
		t.Errorf("Got cap = %d, want %d", got, want)
	}
}
