package compile

import (
	"encoding/binary"

	ops "github.com/go-interpreter/wagon/wasm/operators"
	asm "github.com/twitchyliquid64/golang-asm"
	"github.com/twitchyliquid64/golang-asm/obj"
	"github.com/twitchyliquid64/golang-asm/obj/x86"
)

// Details of the AMD64 backend:
// Reserved registers (for now):
//  - R10 - pointer to stack sliceHeader
//  - R11 - pointer to locals sliceHeader
//  - R12 - pointer for stack item
//  - R13 - stack size
// Scratch registers:
//  - RAX, RBX, RCX, RDX, R8, R9, R14, R15
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
			},
		}
	}
	return b.s
}

// Build implements exec.instructionBuilder.
func (b *AMD64Backend) Build(lower, upper uint, code []byte, meta []InstructionMetadata) ([]byte, error) {
	builder, err := asm.NewBuilder("amd64", 64)
	if err != nil {
		return nil, err
	}
	b.emitPreamble(builder)

	for i := lower; i < upper; i++ {
		switch meta[i].Op {
		case ops.I64Const:
			b.emitPushI64(builder, binary.LittleEndian.Uint64(code[meta[i].Start+1:]))
		case ops.I64Add, ops.I64Sub:
			b.emitBinaryI64(builder, meta[i].Op)
		}
	}

	return builder.Assemble(), nil
}

func (b *AMD64Backend) emitWasmStackLoad(builder *asm.Builder, reg int16) {
	// movq r13,     [r10+8]
	// decq r13
	// movq [r10+8], r13
	// movq r12,     [r10]
	// leaq r12,     [r12 + r13*8]
	// movq reg,     [r12]

	prog := builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R13
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R10
	prog.From.Offset = 8
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.ADECQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R13
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_R13
	prog.To.Type = obj.TYPE_MEM
	prog.To.Reg = x86.REG_R10
	prog.To.Offset = 8
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R10
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.ALEAQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R12
	prog.From.Scale = 8
	prog.From.Index = x86.REG_R13
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R12
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = reg
	builder.AddInstruction(prog)
}

func (b *AMD64Backend) emitWasmStackPush(builder *asm.Builder, reg int16) {
	// movq r13,     [r10+8]
	// incq r13
	// movq [r10+8], r13
	// movq r12,     [r10]
	// leaq r12,     [r12 + r13*8]
	// movq [r12],   reg
	prog := builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R13
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R10
	prog.From.Offset = 8
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AINCQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R13
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_R13
	prog.To.Type = obj.TYPE_MEM
	prog.To.Reg = x86.REG_R10
	prog.To.Offset = 8
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R10
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.ALEAQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_R12
	prog.From.Scale = 8
	prog.From.Index = x86.REG_R13
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_MEM
	prog.To.Reg = x86.REG_R12
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = reg
	builder.AddInstruction(prog)
}

func (b *AMD64Backend) emitBinaryI64(builder *asm.Builder, op byte) {
	b.emitWasmStackLoad(builder, x86.REG_AX)
	b.emitWasmStackLoad(builder, x86.REG_R9)

	prog := builder.NewProg()
	prog.From.Type = obj.TYPE_REG
	prog.From.Reg = x86.REG_R9
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	switch op {
	case ops.I64Add:
		prog.As = x86.AADDQ
	case ops.I64Sub:
		prog.As = x86.ASUBQ
	}
	builder.AddInstruction(prog)

	b.emitWasmStackPush(builder, x86.REG_AX)
}

func (b *AMD64Backend) emitPushI64(builder *asm.Builder, c uint64) {
	prog := builder.NewProg()
	prog.As = x86.AMOVQ
	prog.From.Type = obj.TYPE_CONST
	prog.From.Offset = int64(c)
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_AX
	builder.AddInstruction(prog)
	b.emitWasmStackPush(builder, x86.REG_AX)
}

// emitPreamble loads the address of the stack slice & locals into
// R10 and R11 respectively.
func (b *AMD64Backend) emitPreamble(builder *asm.Builder) {
	prog := builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R10
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_SP
	prog.From.Offset = 8
	builder.AddInstruction(prog)

	prog = builder.NewProg()
	prog.As = x86.AMOVQ
	prog.To.Type = obj.TYPE_REG
	prog.To.Reg = x86.REG_R11
	prog.From.Type = obj.TYPE_MEM
	prog.From.Reg = x86.REG_SP
	prog.From.Offset = 16
	builder.AddInstruction(prog)
}
