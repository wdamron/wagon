// Copyright 2019 The go-interpreter Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !appengine

package compile

import "testing"

func TestMMapAllocator(t *testing.T) {
	a := &MMapAllocator{}
	defer a.Close()

	shortAlloc, err := a.AllocateExec([]byte{1, 2, 3, 4})
	if err != nil {
		t.Fatal(err)
	}
	if d := **(**[4]byte)(shortAlloc.(*asmBlock).mem); d != [4]byte{1, 2, 3, 4} {
		t.Errorf("shortAlloc = %d, want [4]byte{1,2,3,4}", d)
	}
	if want := uint32(128); a.last.consumed != want {
		t.Errorf("a.last.consumed = %d, want %d", a.last.consumed, want)
	}
	if want := uint32(minAllocSize - allocationAlignment - 1); a.last.remaining != want {
		t.Errorf("a.last.remaining = %d, want %d", a.last.remaining, want)
	}

	// Test allocation of massive slice - should be 32k more & new block.
	b := make([]byte, 36*1024)
	b[1] = 5
	massiveAlloc, err := a.AllocateExec(b)
	if err != nil {
		t.Fatal(err)
	}
	if d := **(**[2]byte)(massiveAlloc.(*asmBlock).mem); d != [2]byte{0, 5} {
		t.Errorf("bigAlloc = %d, want [2]byte{31, 0}", d)
	}
	if want := uint32(36 * 1024); a.last.consumed != want {
		t.Errorf("a.last.consumed = %d, want %d", a.last.consumed, want)
	}
	if want := uint32(minAllocSize); a.last.remaining != want {
		t.Errorf("a.last.remaining = %d, want %d", a.last.remaining, want)
	}
}
