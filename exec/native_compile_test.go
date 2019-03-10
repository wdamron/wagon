package exec

import (
	"bytes"
	"runtime"
	"testing"
	"unsafe"

	"github.com/go-interpreter/wagon/disasm"
	"github.com/go-interpreter/wagon/exec/internal/compile"
	ops "github.com/go-interpreter/wagon/wasm/operators"
)

var oldBackends []compilerVariant

func setupNativeAsmBackendMocks(t *testing.T) (*mockSequenceScanner, *mockPageAllocator, *mockInstructionBuilder) {
	t.Helper()
	oldBackends = supportedNativeArchPlatforms
	supportedNativeArchPlatforms = []compilerVariant{
		{
			OS:   runtime.GOOS,
			Arch: runtime.GOARCH,
		},
	}
	s := &mockSequenceScanner{}
	supportedNativeArchPlatforms[0].Scanner = s
	p := &mockPageAllocator{}
	supportedNativeArchPlatforms[0].PageAllocator = p
	b := &mockInstructionBuilder{}
	supportedNativeArchPlatforms[0].Builder = b
	return s, p, b
}

type mockSequenceScanner struct {
	emit []compile.CompilationCandidate
}

func (s *mockSequenceScanner) ScanFunc(bc []byte, meta []compile.InstructionMetadata) ([]compile.CompilationCandidate, error) {
	return s.emit, nil
}

type mockPageAllocator struct{}

func (a *mockPageAllocator) AllocateExec(asm []byte) (unsafe.Pointer, error) {
	return unsafe.Pointer(&asm), nil
}

type mockInstructionBuilder struct{}

func (b *mockInstructionBuilder) Build(lower, upper uint, code []byte) ([]byte, error) {
	return []byte{byte(lower), byte(upper)}, nil
}

func TestNativeAsmStructureSetup(t *testing.T) {
	scanner, _, _ := setupNativeAsmBackendMocks(t)
	defer func() {
		supportedNativeArchPlatforms = oldBackends
	}()

	constInst, _ := ops.New(ops.I32Const)
	addInst, _ := ops.New(ops.I32Add)
	subInst, _ := ops.New(ops.I32Sub)
	setGlobalInst, _ := ops.New(ops.SetGlobal)

	wasm, err := disasm.Assemble([]disasm.Instr{
		{Op: constInst, Immediates: []interface{}{int32(1)}},
		{Op: constInst, Immediates: []interface{}{int32(1)}},
		{Op: addInst},
		{Op: setGlobalInst, Immediates: []interface{}{uint32(0)}},

		{Op: constInst, Immediates: []interface{}{int32(8)}},
		{Op: constInst, Immediates: []interface{}{int32(16)}},
		{Op: constInst, Immediates: []interface{}{int32(4)}},
		{Op: addInst},
		{Op: subInst},
	})
	if err != nil {
		t.Fatal(err)
	}

	vm := &VM{
		funcs: []function{
			compiledFunction{
				code: wasm,
			},
		},
	}
	vm.newFuncTable()

	// setup mocks.
	scanner.emit = []compile.CompilationCandidate{
		// Sequence with single integer op - should not compiled.
		compile.CompilationCandidate{Beginning: 0, End: 7, Metrics: &compile.Metrics{IntegerOps: 1}},
		// Sequence with two integer ops - should be emitted.
		compile.CompilationCandidate{Beginning: 7, End: 15, Metrics: &compile.Metrics{IntegerOps: 2}},
	}

	if err := vm.tryNativeCompile(); err != nil {
		t.Fatalf("tryNativeCompile() failed: %v", err)
	}

	// Our scanner emitted two sequences. The first should not have resulted in
	// compilation, but the second should have. Lets check thats the case.
	fn := vm.funcs[0].(compiledFunction)
	if want := 1; len(fn.asm) != want {
		t.Fatalf("len(fn.asm) = %d, want %d", len(vm.funcs[0].(compiledFunction).asm), want)
	}
	if want := 8; int(fn.asm[0].stride) != want {
		t.Errorf("fn.asm[0].stride = %v, want %v", fn.asm[0].stride, want)
	}

	// The function bytecode should have been modified to call wagon.nativeExec,
	// with the index of the block (0) following, and remaining bytes set to the
	// unreachable opcode.
	if want := ops.WagonNativeExec; fn.code[7] != want {
		t.Errorf("fn.code[7] = %v, want %v", fn.code[7], want)
	}
	if want := []byte{0, 0, 0, 0}; !bytes.Equal(fn.code[8:12], want) {
		t.Errorf("fn.code[8:12] = %v, want %v", fn.code[8:12], want)
	}
	for i := 13; i < len(fn.code); i++ {
		if fn.code[i] != ops.Unreachable {
			t.Errorf("fn.code[%d] = %v, want ops.Unreachable", i, fn.code[i])
		}
	}
}
