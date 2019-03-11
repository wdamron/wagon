package compile

import (
	ops "github.com/go-interpreter/wagon/wasm/operators"
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
	// Bytecode index of the first opcode.
	Beginning uint
	// Bytecode index of the last byte in the instruction.
	End uint
	// InstructionMeta index of the first instruction.
	StartInstruction int
	// InstructionMeta index of the last instruction.
	EndInstruction int
	// Metrics about the instructions between first & last index.
	Metrics *Metrics
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

// ScanFunc scans the given function information, emitting selections of
// bytecode which could be compiled into function code.
func (s *scanner) ScanFunc(bytecode []byte, meta *BytecodeMetadata) ([]CompilationCandidate, error) {
	var finishedCandidates []CompilationCandidate

	inProgress := CompilationCandidate{End: 1, Metrics: &Metrics{}}

	for i, inst := range meta.Instructions {
		// Except for the first instruction, we cant emit a native section
		// where other parts of code try and call into us halfway. Maybe we
		// can support that in the future.
		isInsideBranchTarget := meta.InboundTargets[int64(inst.Start)] && inst.Start > 0

		if !s.supportedOpcodes[inst.Op] || isInsideBranchTarget {
			//fmt.Printf("not supported: 0x%x\n", inst.Op)
			// See if the candidate can be emitted.
			if inProgress.Metrics.AllOps > 2 {
				finishedCandidates = append(finishedCandidates, inProgress)
			}
			nextOp := uint(inst.Start + inst.Size)
			inProgress = CompilationCandidate{
				Beginning:        nextOp,
				End:              nextOp,
				StartInstruction: i,
				EndInstruction:   i,
				Metrics:          &Metrics{},
			}
		} else {
			// Still a run of supported instructions - increment end
			// cursor of current candidate.
			inProgress.End = uint(inst.Start) + uint(inst.Size)
			inProgress.EndInstruction++
		}

		// TODO: Add to this table as backends support more opcodes.
		switch inst.Op {
		case ops.I64Const:
			inProgress.Metrics.IntegerOps++
			inProgress.Metrics.StackWrites++
		case ops.I64Add, ops.I64Sub:
			inProgress.Metrics.IntegerOps++
			inProgress.Metrics.StackReads += 2
			inProgress.Metrics.StackWrites++
		}
		inProgress.Metrics.AllOps++
	}

	if inProgress.Metrics.AllOps > 2 {
		inProgress.End++
		finishedCandidates = append(finishedCandidates, inProgress)
	}

	//fmt.Printf("Candidates: %+v\n", finishedCandidates)
	//fmt.Printf("Instructions: %+v\n", meta.Instructions)
	return finishedCandidates, nil
}
