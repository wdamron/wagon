package native

type scanner struct {
	supportedOpcodes map[byte]bool
}

func (s *scanner) ScanFunc(fn *compiledFunction) ([]candidateSequence, error) {
	return nil, nil
}
