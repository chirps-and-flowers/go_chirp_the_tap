package loaders

// IsTurbotape250Lead checks if the data at startPos matches the Turbotape 250 lead tone signature.
// It looks for a sequence of 0x02 bytes (pilot), followed by 0x09 (sync),
// followed by the 9-byte sequence (9 8 7 6 5 4 3 2 1).
// Returns true if the signature is found, along with the length of the detected lead block.
func IsTurbotape250Lead(tapData []byte, startPos int, defaultTolerance, pulsesInByte int) (bool, int, int, int, int) {
	// Turbotape 250 format parameters (from loaders.Ft array)
	turbotapeFmt := Ft[TT_HEAD] // Use TT_HEAD for parameters

	lp := turbotapeFmt.LP
	sp := turbotapeFmt.SP
	tp := turbotapeFmt.TP
	en := turbotapeFmt.Endian

	currentPos := startPos

	// 1. Find pilot tone
	pilotEndPos, pilotFound, err := findPilot(tapData, currentPos, TT_HEAD, defaultTolerance)
	if err != nil || !pilotFound {
		return false, 0, 0, 0, 0
	}
	currentPos = pilotEndPos

	// 2. Check for sync and countdown sequence (9 8 7 6 5 4 3 2 1)
	expectedSequence := []byte{9, 8, 7, 6, 5, 4, 3, 2, 1}
	for _, expectedByte := range expectedSequence {
		if currentPos+pulsesInByte > len(tapData) {
			return false, 0, 0, 0, 0
		}
		val, err := ReadByte(tapData, currentPos, lp, sp, tp, en, defaultTolerance)
		if err != nil || byte(val) != expectedByte {
			return false, 0, 0, 0, 0
		}
		currentPos += pulsesInByte // Consume sequence byte
	}

	// 4. Read blockID and calculate psize if it's a header
	blockIDPos := currentPos
	if blockIDPos+pulsesInByte > len(tapData) {
		return false, 0, 0, 0, 0
	}
	blockID, err := ReadByte(tapData, blockIDPos, lp, sp, tp, en, defaultTolerance)
	if err != nil {
		return false, 0, 0, 0, 0
	}
	currentPos += pulsesInByte // Consume blockID byte

	psize := 0
	if blockID == 1 || blockID == 2 { // It's a header
		// Read start address (hd[1], hd[2])
		if currentPos+pulsesInByte > len(tapData) {
			return false, 0, 0, 0, 0
		}
		startAddrLow, err := ReadByte(tapData, currentPos, lp, sp, tp, en, defaultTolerance)
		if err != nil {
			return false, 0, 0, 0, 0
		}
		currentPos += pulsesInByte
		if currentPos+pulsesInByte > len(tapData) {
			return false, 0, 0, 0, 0
		}
		startAddrHigh, err := ReadByte(tapData, currentPos, lp, sp, tp, en, defaultTolerance)
		if err != nil {
			return false, 0, 0, 0, 0
		}
		currentPos += pulsesInByte
		startAddr := startAddrLow + (startAddrHigh << 8)

		// Read end address (hd[3], hd[4])
		if currentPos+pulsesInByte > len(tapData) {
			return false, 0, 0, 0, 0
		}
		endAddrLow, err := ReadByte(tapData, currentPos, lp, sp, tp, en, defaultTolerance)
		if err != nil {
			return false, 0, 0, 0, 0
		}
		currentPos += pulsesInByte
		if currentPos+pulsesInByte > len(tapData) {
			return false, 0, 0, 0, 0
		}
		endAddrHigh, err := ReadByte(tapData, currentPos, lp, sp, tp, en, defaultTolerance)
		if err != nil {
			return false, 0, 0, 0, 0
		}
		currentPos += pulsesInByte
		endAddr := endAddrLow + (endAddrHigh << 8)

		psize = (endAddr - startAddr) + 1

		// Read filename (16 bytes)
		filenameEndPos := currentPos + (16 * pulsesInByte)
		for currentPos < filenameEndPos {
			if currentPos >= len(tapData) {
				goto end_of_header_processing
			}
			if currentPos+pulsesInByte > len(tapData) {
				goto end_of_header_processing
			}
			_, err := ReadByte(tapData, currentPos, lp, sp, tp, en, defaultTolerance)
			if err != nil {
				goto end_of_header_processing
			}
			currentPos += pulsesInByte
		}

		// Scan through padding bytes (0x20)
		for currentPos < len(tapData) {
			if currentPos+pulsesInByte > len(tapData) {
				break
			}
			val, err := ReadByte(tapData, currentPos, lp, sp, tp, en, defaultTolerance)
			if err != nil || byte(val) != 0x20 {
				break
			}
			currentPos += pulsesInByte // Consume the padding byte
		}
	}

		// Read checksum byte
		if currentPos+pulsesInByte > len(tapData) {
			goto end_of_header_processing
		}
		_, err = ReadByte(tapData, currentPos, lp, sp, tp, en, defaultTolerance)
		if err != nil {
			goto end_of_header_processing
		}
		currentPos += pulsesInByte

end_of_header_processing:
	// If all checks pass, it's a Turbotape 250 lead
	return true, currentPos - startPos, blockID, psize, pilotEndPos
}

