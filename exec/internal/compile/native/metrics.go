package native

// Metrics describes the heuristics of an instruction sequence.
type Metrics struct {
	MemoryReads, MemoryWrites uint
	StackReads, StackWrites   uint

	IntegerOps, FloatOps uint
}
