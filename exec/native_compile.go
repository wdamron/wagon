package exec

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/go-interpreter/wagon/exec/internal/compile/native"
	ops "github.com/go-interpreter/wagon/wasm/operators"
)

// Parameters which decide when a sequence should be compiled.
// TODO: Expose some way for these to be customized at runtime
// via VMOptions.
const (
	// NOTE: must never be less than 5, as room is needed to pack the
	// wagon.nativeExec instruction and its parameter.
	minInstructionSequence      = 5
	minArithInstructionSequence = 2

	nativeExecPrologueSize = 5
)

var (
	supportedNativeArchPlatforms = []compilerVariant{}
)

// compilerVariant parameterizes backends for native compilation.
type compilerVariant struct {
	Arch, OS      string
	PageAllocator pageAllocator
	Scanner       sequenceScanner
	Builder       instructionBuilder
}

// pageAllocator is responsible for the efficient allocation of
// executable, aligned regions of executable memory.
type pageAllocator interface {
	AllocateExec(asm []byte) (unsafe.Pointer, error)
}

// sequenceScanner is responsible for detecting runs of supported opcodes
// that could benefit from compilation into native instructions.
type sequenceScanner interface {
	// ScanFunc returns an ordered, non-overlapping set of
	// sequences to compile into native code.
	ScanFunc(fn *compiledFunction) ([]candidateSequence, error)
}

// candidateSequence represents a series of instructions which are candidates
// for conversion into native code.
type candidateSequence interface {
	// Start & End bounds of the sequence in the wasm instruction slice.
	Bounds() (uint, uint)
	// Metrics returns heuristic information about instructions in the
	// sequence.
	Metrics() *native.Metrics
}

// instructionBuilder is responsible for compiling wasm opcodes into
// native instructions.
type instructionBuilder interface {
	// Build compiles the specified bytecode into native instructions.
	Build(lower, upper uint, code []byte) ([]byte, error)
}

func nativeBackend() (bool, *compilerVariant) {
	for _, c := range supportedNativeArchPlatforms {
		if c.Arch == runtime.GOARCH && c.OS == runtime.GOOS {
			return true, &c
		}
	}
	return false, nil
}

func (vm *VM) tryNativeCompile() error {
	supportedPlatform, backend := nativeBackend()
	if !supportedPlatform {
		return nil
	}

	// TODO(twitchyliquid64): Parallelize scanning + compilation at the function
	// level for some speed gains.
	for i := range vm.funcs {
		if _, isGoFunc := vm.funcs[i].(*goFunction); isGoFunc {
			continue
		}

		fn := vm.funcs[i].(*compiledFunction)
		candidates, err := backend.Scanner.ScanFunc(fn)
		if err != nil {
			return fmt.Errorf("AOT scan failed on vm.funcs[%d]: %v", i, err)
		}

		for _, candidate := range candidates {
			m := candidate.Metrics()
			if (m.IntegerOps + m.FloatOps) < minArithInstructionSequence {
				continue
			}
			lower, upper := candidate.Bounds()
			if (upper - lower) < minInstructionSequence {
				continue
			}

			asm, err := backend.Builder.Build(lower, upper, fn.code)
			if err != nil {
				return fmt.Errorf("native compilation failed on vm.funcs[%d].code[%d:%d]: %v", i, lower, upper, err)
			}
			addr, err := backend.PageAllocator.AllocateExec(asm)
			if err != nil {
				return fmt.Errorf("PageAllocator.AllocateExec() failed: %v", err)
			}
			fn.asm = append(fn.asm, asmBlock{
				addr:   addr,
				stride: upper - lower,
			})

			// Patch the wasm opcode stream to call into the native section.
			// The number of bytes touched here must always be equal to
			// nativeExecPrologueSize and <= minInstructionSequence.
			fn.code[lower] = ops.WagonNativeExec
			endianess.PutUint32(fn.code[lower+1:], uint32(len(fn.asm)-1))
			// make the remainder of the recompiled instructions
			// unreachable: this should trap the program in the event that
			// a bug in code offsets & candidate sequence detection results in
			// a jump to the middle of re-compiled code.
			// This conservative behaviour is the least likely to result in
			// bugs becoming security issues.
			for i := lower + 5; i < upper; i++ {
				fn.code[i] = ops.Unreachable
			}
		}
	}

	return nil
}

// nativeCodeInvocation calls into one of the assembled code blocks.
// Assembled code blocks expect the following two pieces of
// information on the stack:
// [fp:fp+pointerSize]: sliceHeader for the stack.
// [fp+pointerSize:fp+pointerSize*2]: sliceHeader for locals variables.
func (vm *VM) nativeCodeInvocation(asmIndex uint32) {
	block := (*vm.ctx.asm)[asmIndex]
	fp := *(*func(unsafe.Pointer, unsafe.Pointer))(block.addr)
	fp(unsafe.Pointer(&vm.ctx.stack), unsafe.Pointer(&vm.ctx.locals))
	vm.ctx.pc += int64(block.stride - nativeExecPrologueSize)
}
