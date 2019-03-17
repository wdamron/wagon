// Copyright 2019 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compile

import (
	"encoding/binary"
	"fmt"

	ops "github.com/go-interpreter/wagon/wasm/operators"

	// Importing everything into the current scope makes for less noise:
	. "github.com/wdamron/x64"
)

// NativeCodeUnit represents compiled native code.
type NativeCodeUnit interface {
	Invoke(stack, locals *[]uint64)
}

// dirtyRegs hold booleans that are true when the register stores
// a reserved value that needs to be flushed to memory.
type dirtyRegs struct {
	R12 bool
	R13 bool
}

// Details of the AMD64 backend:
// Reserved registers (for now):
//  - R10 - pointer to stack sliceHeader
//  - R11 - pointer to locals sliceHeader
//  - R12 - pointer for stack item
//  - R13 - stack size
// Scratch registers:
//  - RAX, RBX, RCX, RDX, R8, R9, R15
// Most emission instructions make few attempts to optimize in order
// to keep things simple, however a planned second pass peephole-optimizer
//  should make a big difference.

// AMD64Backend is the native compiler backend for x86-64 architectures.
type AMD64Backend struct {
	s *scanner
}

// Scanner returns a scanner that can be used for
// emitting compilation candidates.
func (b *AMD64Backend) Scanner() *scanner {
	if b.s == nil {
		b.s = &scanner{
			supportedOpcodes: map[byte]bool{
				ops.I64Const: true,
				ops.I64Add:   true,
				ops.I64Sub:   true,
				ops.I64And:   true,
				ops.I64Or:    true,
				ops.I64Mul:   true,
				ops.GetLocal: true,
			},
		}
	}
	return b.s
}

// Build implements exec.instructionBuilder.
func (b *AMD64Backend) Build(candidate CompilationCandidate, code []byte, meta *BytecodeMetadata) ([]byte, error) {
	a := NewAssembler(make([]byte, 1024))

	var regs dirtyRegs
	b.emitPreamble(a, &regs)

	for i := candidate.StartInstruction; i <= candidate.EndInstruction; i++ {
		//fmt.Printf("i=%d, meta=%+v, len=%d\n", i, meta.Instructions[i], len(code))
		inst := meta.Instructions[i]
		switch inst.Op {
		case ops.I64Const:
			b.emitPushI64(a, &regs, b.readIntImmediate(code, inst))
		case ops.GetLocal:
			b.emitWasmLocalsLoad(a, &regs, RAX, b.readIntImmediate(code, inst))
			b.emitWasmStackPush(a, &regs, RAX)
		case ops.I64Add, ops.I64Sub, ops.I64Mul, ops.I64Or, ops.I64And:
			if err := b.emitBinaryI64(a, &regs, inst.Op); err != nil {
				return nil, fmt.Errorf("emitBinaryI64: %v", err)
			}
		default:
			return nil, fmt.Errorf("cannot handle inst[%d].Op 0x%x", i, inst.Op)
		}
	}
	b.emitPostamble(a, &regs)
	a.Finalize()

	return a.Code(), a.Err()
}

func (b *AMD64Backend) readIntImmediate(code []byte, meta InstructionMetadata) uint64 {
	if meta.Size == 5 {
		return uint64(binary.LittleEndian.Uint32(code[meta.Start+1 : meta.Start+meta.Size]))
	}
	return binary.LittleEndian.Uint64(code[meta.Start+1 : meta.Start+meta.Size])
}

func (b *AMD64Backend) emitWasmLocalsLoad(a *Assembler, regs *dirtyRegs, reg Reg, index uint64) {
	a.RI(MOV, RBX, Imm64(index))
	a.RM(MOV, RCX, Mem{Base: R11})
	a.RM(LEA, RCX, Mem{Base: RCX, Index: RBX, Scale: 8})
	a.RR(MOV, reg, RCX)
}

func (b *AMD64Backend) emitWasmStackLoad(a *Assembler, regs *dirtyRegs, reg Reg) {
	if !regs.R13 {
		a.RM(MOV, R13, Mem{Base: R10, Disp: Rel8(8)})
	}

	a.Inst(DEC, R13)

	if !regs.R12 {
		a.RM(MOV, R12, Mem{Base: R10})
	}

	a.RM(LEA, R12, Mem{Base: R12, Index: R13, Scale: 8})
	a.RM(MOV, reg, Mem{Base: R12})
}

func (b *AMD64Backend) emitWasmStackPush(a *Assembler, regs *dirtyRegs, reg Reg) {
	if !regs.R13 {
		a.RM(MOV, R13, Mem{Base: R10, Disp: Rel8(8)})
	}

	if !regs.R12 {
		a.RM(MOV, R12, Mem{Base: R10})
	}

	a.RM(LEA, R12, Mem{Base: R12, Index: R13, Scale: 8})
	a.MR(MOV, Mem{Base: R12}, reg)
	a.Inst(INC, R13)
}

func (b *AMD64Backend) emitBinaryI64(a *Assembler, regs *dirtyRegs, op byte) error {
	b.emitWasmStackLoad(a, regs, R9)
	b.emitWasmStackLoad(a, regs, RAX)

	switch op {
	case ops.I64Add:
		a.RR(ADD, RAX, R9)
	case ops.I64Sub:
		a.RR(SUB, RAX, R9)
	case ops.I64And:
		a.RR(AND, RAX, R9)
	case ops.I64Or:
		a.RR(OR, RAX, R9)
	case ops.I64Mul:
		a.Inst(MUL, R9)
	default:
		return fmt.Errorf("cannot handle op: %x", op)
	}

	b.emitWasmStackPush(a, regs, RAX)
	return nil
}

func (b *AMD64Backend) emitPushI64(a *Assembler, regs *dirtyRegs, c uint64) {
	a.RI(MOV, RAX, Imm64(c))
	b.emitWasmStackPush(a, regs, RAX)
}

// emitPreamble loads the address of the stack slice & locals into
// R10 and R11 respectively.
func (b *AMD64Backend) emitPreamble(a *Assembler, regs *dirtyRegs) {
	a.RM(MOV, R10, Mem{Base: RSP, Disp: Rel8(8)})
	a.RM(MOV, R11, Mem{Base: RSP, Disp: Rel8(16)})
}

func (b *AMD64Backend) emitPostamble(a *Assembler, regs *dirtyRegs) {
	if regs.R13 {
		a.MR(MOV, Mem{Base: R10, Disp: Rel8(8)}, R13)
	}

	a.Inst(RET)
}
