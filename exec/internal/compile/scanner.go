package compile

import (
	ops "github.com/go-interpreter/wagon/wasm/operators"
)

var (
	// AMD64Backend is the native compiler backend for x86-64 architectures.
	AMD64Backend = &scanner{}
)

type scanner struct {
	supportedOpcodes map[byte]bool
}

// InstructionMetadata describes a bytecode instruction.
type InstructionMetadata struct {
	Op    byte
	Start int
	Size  int
}

// CompilationCandidate describes a range of bytecode that can
// be translated to native code.
type CompilationCandidate struct {
	Beginning uint
	End       uint
	Metrics   *Metrics
}

// Bounds returns the beginning & end index in the bytecode which
// this candidate would replace.
func (s *CompilationCandidate) Bounds() (uint, uint) {
	return s.Beginning, s.End
}

// Metrics describes the heuristics of an instruction sequence.
type Metrics struct {
	MemoryReads, MemoryWrites uint
	StackReads, StackWrites   uint

	AllOps     int
	IntegerOps int
	FloatOps   int
}

// ScanFunc implements exec.sequenceScanner.
func (s *scanner) ScanFunc(bytecode []byte, meta []InstructionMetadata) ([]CompilationCandidate, error) {
	var finishedCandidates []CompilationCandidate

	inProgress := CompilationCandidate{Metrics: &Metrics{}}

	for _, inst := range meta {
		if !s.supportedOpcodes[inst.Op] {
			// See if the candidate can be emitted.
			if inProgress.Beginning+1 < inProgress.End && inProgress.Metrics.AllOps > 2 {
				finishedCandidates = append(finishedCandidates, inProgress)
			}
			nextOp := uint(inst.Start + inst.Size)
			inProgress = CompilationCandidate{Beginning: nextOp, End: nextOp, Metrics: &Metrics{}}
		}

		// Still a run of supported instructions - increment end
		// cursor of current candidate.
		inProgress.End += uint(inst.Size)

		// TODO: Add to this table as backends support more opcodes.
		switch inst.Op {
		case ops.I64Const:
			inProgress.Metrics.IntegerOps++
			inProgress.Metrics.AllOps++
			inProgress.Metrics.StackWrites++
		case ops.I64Add, ops.I64Sub:
			inProgress.Metrics.IntegerOps++
			inProgress.Metrics.AllOps++
			inProgress.Metrics.StackReads += 2
			inProgress.Metrics.StackWrites++
		}
	}
	// var inProgress CompilationCandidate

	return finishedCandidates, nil
}
