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
	if runtime.GOARCH != "amd64" || runtime.GOOS != "linux" {
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

	ptr, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 0, 5)
	fakeLocals := make([]uint64, 0, 0)
	f := (uintptr)(unsafe.Pointer(&ptr))
	fp := **(**func(unsafe.Pointer, unsafe.Pointer))(unsafe.Pointer(&f))
	fp(unsafe.Pointer(&fakeStack), unsafe.Pointer(&fakeLocals))

	if want := 2; len(fakeStack) != want {
		t.Errorf("fakeStack.Len = %d, want %d", len(fakeStack), want)
	}
	if want := uint64(1234); fakeStack[0] != want {
		t.Errorf("fakeStack[0] = %d, want %d", fakeStack[0], want)
	}
	if want := uint64(5678); fakeStack[1] != want {
		t.Errorf("fakeStack[1] = %d, want %d", fakeStack[1], want)
	}
}

func TestAMD64StackPop(t *testing.T) {
	if runtime.GOARCH != "amd64" || runtime.GOOS != "linux" {
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

	ptr, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 2, 5)
	fakeStack[1] = 1337
	fakeLocals := make([]uint64, 0, 0)
	f := (uintptr)(unsafe.Pointer(&ptr))
	fp := **(**func(unsafe.Pointer, unsafe.Pointer))(unsafe.Pointer(&f))
	fp(unsafe.Pointer(&fakeStack), unsafe.Pointer(&fakeLocals))

	if want := 1; len(fakeStack) != want {
		t.Errorf("fakeStack.Len = %d, want %d", len(fakeStack), want)
	}
	if want := uint64(1337); fakeStack[0] != want {
		t.Errorf("fakeStack[0] = %d, want %d", fakeStack[0], want)
	}
}

func TestAMD64ConstAdd(t *testing.T) {
	if runtime.GOARCH != "amd64" || runtime.GOOS != "linux" {
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

	ptr, err := allocator.AllocateExec(out)
	if err != nil {
		t.Fatal(err)
	}

	fakeStack := make([]uint64, 1, 5)
	fakeStack[0] = 12
	fakeLocals := make([]uint64, 0, 0)
	f := (uintptr)(unsafe.Pointer(&ptr))
	fp := **(**func(unsafe.Pointer, unsafe.Pointer))(unsafe.Pointer(&f))
	fp(unsafe.Pointer(&fakeStack), unsafe.Pointer(&fakeLocals))

	if want := 1; len(fakeStack) != want {
		t.Fatalf("fakeStack.Len = %d, want %d", len(fakeStack), want)
	}
	if want := uint64(15); fakeStack[0] != want {
		t.Errorf("fakeStack[0] = %d, want %d", fakeStack[0], want)
	}
}

func TestSliceMemoryLayoutAMD64(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.SkipNow()
	}

	slice := make([]uint64, 2, 5)
	mem := (*[24]byte)(unsafe.Pointer(&slice))
	if want := uint64(2); binary.LittleEndian.Uint64(mem[8:16]) != want {
		t.Errorf("Got len = %d, want %d", binary.LittleEndian.Uint64(mem[8:16]), want)
	}
	if want := uint64(5); binary.LittleEndian.Uint64(mem[16:24]) != want {
		t.Errorf("Got cap = %d, want %d", binary.LittleEndian.Uint64(mem[16:24]), want)
	}
}
