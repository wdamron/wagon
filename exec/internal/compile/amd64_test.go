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
	b.emitPreamble(builder)
	mov := builder.NewProg()
	mov.As = x86.AMOVQ
	mov.From.Type = obj.TYPE_CONST
	mov.From.Offset = 1234
	mov.To.Type = obj.TYPE_REG
	mov.To.Reg = x86.REG_AX
	builder.AddInstruction(mov)
	b.emitWasmStackPush(builder, x86.REG_AX)
	mov = builder.NewProg()
	mov.As = x86.AMOVQ
	mov.From.Type = obj.TYPE_CONST
	mov.From.Offset = 5678
	mov.To.Type = obj.TYPE_REG
	mov.To.Reg = x86.REG_AX
	builder.AddInstruction(mov)
	b.emitWasmStackPush(builder, x86.REG_AX)
	ret := builder.NewProg()
	ret.As = obj.ARET
	builder.AddInstruction(ret)
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
	b.emitPreamble(builder)
	b.emitWasmStackLoad(builder, x86.REG_AX)
	b.emitWasmStackLoad(builder, x86.REG_BX)
	b.emitWasmStackPush(builder, x86.REG_AX)
	ret := builder.NewProg()
	ret.As = obj.ARET
	builder.AddInstruction(ret)
	out := builder.Assemble()

	// cmd := exec.Command("ndisasm", "-b64", "-")
	// cmd.Stdin = bytes.NewReader(out)
	// cmd.Stdout = os.Stdout
	// cmd.Run()

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

func TestAMD64ConstAdd(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.SkipNow()
	}
	allocator := &MMapAllocator{}
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		t.Fatal(err)
	}

	b := &AMD64Backend{}
	b.emitPreamble(builder)
	b.emitPushI64(builder, 3)
	b.emitBinaryI64(builder, ops.I64Add)
	ret := builder.NewProg()
	ret.As = obj.ARET
	builder.AddInstruction(ret)
	out := builder.Assemble()

	// cmd := exec.Command("ndisasm", "-b64", "-")
	// cmd.Stdin = bytes.NewReader(out)
	// cmd.Stdout = os.Stdout
	// cmd.Run()

	nativeBlock, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 1, 5)
	fakeStack[0] = 12
	fakeLocals := make([]uint64, 0, 0)
	nativeBlock.Invoke(&fakeStack, &fakeLocals)

	if got, want := len(fakeStack), 1; got != want {
		t.Fatalf("fakeStack.Len = %d, want %d", got, want)
	}
	if got, want := fakeStack[0], uint64(15); got != want {
		t.Errorf("fakeStack[0] = %d, want %d", got, want)
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
