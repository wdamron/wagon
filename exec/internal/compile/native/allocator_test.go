package native

import "testing"

func TestMMapAllocator(t *testing.T) {
	a := &MMapAllocator{}

	shortAlloc, err := a.AllocateExec([]byte{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	if d := *(*[4]byte)(shortAlloc); d != [4]byte{1, 2, 3, 4} {
		t.Errorf("shortAlloc = %d, want [4]byte{1,2,3,4}", d)
	}
	if want := uint32(2048); a.last.consumed != want {
		t.Errorf("a.last.consumed = %d, want %d", a.last.consumed, want)
	}
	if want := uint32(minAllocSize - allocationAlignment - 1); a.last.remaining != want {
		t.Errorf("a.last.remaining = %d, want %d", a.last.remaining, want)
	}

	secondAlloc, err := a.AllocateExec([]byte{5, 6, 7, 8, 9})
	if err != nil {
		t.Fatal(err)
	}
	if d := *(*[5]byte)(secondAlloc); d != [5]byte{5, 6, 7, 8, 9} {
		t.Errorf("secondAlloc = %d, want [4]byte{1,2,3,4}", d)
	}
	if want := uint32(2048 * 2); a.last.consumed != want {
		t.Errorf("a.last.consumed = %d, want %d", a.last.consumed, want)
	}
	if want := uint32(minAllocSize - allocationAlignment*2 - 2); a.last.remaining != want {
		t.Errorf("a.last.remaining = %d, want %d", a.last.remaining, want)
	}

	// Allocate the remainder.
	b := make([]byte, 32*1024-2)
	b[0] = 31
	bigAlloc, err := a.AllocateExec(b)
	if err != nil {
		t.Fatal(err)
	}
	if d := *(*[2]byte)(bigAlloc); d != [2]byte{31, 0} {
		t.Errorf("bigAlloc = %d, want [2]byte{31, 0}", d)
	}
	if want := uint32(2048 * 16); a.last.consumed != want {
		t.Errorf("a.last.consumed = %d, want %d", a.last.consumed, want)
	}
	if want := uint32(0); a.last.remaining != want {
		t.Errorf("a.last.remaining = %d, want %d", a.last.remaining, want)
	}
	lastBlock := a.last

	// Test allocation of massive slice - should be 32k more & new block.
	b = make([]byte, 36*1024)
	b[1] = 5
	massiveAlloc, err := a.AllocateExec(b)
	if err != nil {
		t.Fatal(err)
	}
	if lastBlock == a.last {
		t.Error("lastBlock == a.last, want inequality")
	}
	if d := *(*[2]byte)(massiveAlloc); d != [2]byte{0, 5} {
		t.Errorf("bigAlloc = %d, want [2]byte{31, 0}", d)
	}
	if want := uint32(36 * 1024); a.last.consumed != want {
		t.Errorf("a.last.consumed = %d, want %d", a.last.consumed, want)
	}
	if want := uint32(minAllocSize); a.last.remaining != want {
		t.Errorf("a.last.remaining = %d, want %d", a.last.remaining, want)
	}
}
