package loaders

import (
	"fmt"
)

const (
	PULSES_IN_CBM_BYTE = 20
)

// cbm_readbit reads two pulses and interprets them as a CBM bit.
func cbm_readbit(tapData []byte, pos int, sp, mp, lp, tol int) (int, error) {
	const (
		PULSE_S = 0
		PULSE_M = 1
		PULSE_L = 2
	)

	// DEBUG: Print pulse values and thresholds
    p1 := int(tapData[pos])
	p2 := int(tapData[pos+1])

	var b1, b2 int
	b1amb, b2amb := 0, 0

	// Resolve pulse 1
	if p1 > sp-tol && p1 < sp+tol { b1 = PULSE_S; b1amb |= 1 }
	if p1 > mp-tol && p1 < mp+tol { b1 = PULSE_M; b1amb |= 2 }
	if p1 > lp-tol && p1 < lp+tol { b1 = PULSE_L; b1amb |= 4 }

	// Resolve pulse 2
	if p2 > sp-tol && p2 < sp+tol { b2 = PULSE_S; b2amb |= 1 }
	if p2 > mp-tol && p2 < mp+tol { b2 = PULSE_M; b2amb |= 2 }
	if p2 > lp-tol && p2 < lp+tol { b2 = PULSE_L; b2amb |= 4 }

	// Handle ambiguities
	if b1amb == 3 { // S and M
		if abs(p1-sp) < abs(p1-mp) { b1 = PULSE_S } else { b1 = PULSE_M }
	}
	if b1amb == 6 { // M and L
		if abs(p1-mp) < abs(p1-lp) { b1 = PULSE_M } else { b1 = PULSE_L }
	}
	if b2amb == 3 { // S and M
		if abs(p2-sp) < abs(p2-mp) { b2 = PULSE_S } else { b2 = PULSE_M }
	}
	if b2amb == 6 { // M and L
		if abs(p2-mp) < abs(p2-lp) { b2 = PULSE_M } else { b2 = PULSE_L }
	}

	// If after ambiguity resolution, either pulse is still ambiguous (more than one bit set), it's unreadable.
	// This checks if b1amb or b2amb is not a power of 2 (1, 2, or 4).
	if (b1amb & (b1amb-1)) != 0 || (b2amb & (b2amb-1)) != 0 {
		return -1, fmt.Errorf("unreadable signal due to unresolved ambiguity")
	}


	switch {
	case b1 == PULSE_S && b2 == PULSE_M: return 0, nil // 0 bit
	case b1 == PULSE_M && b2 == PULSE_S: return 1, nil // 1 bit
	case b1 == PULSE_L && b2 == PULSE_M: return 2, nil // New data marker
	case b1 == PULSE_L && b2 == PULSE_S: return 3, nil // End of data marker
	default: return -1, fmt.Errorf("invalid cbm pulse pair")
	}
}

// cbm_readbyte reads a CBM-encoded byte.
func cbm_readbyte(tapData []byte, pos int, sp, mp, lp, tol int) (int, int, error) {
		if pos < PULSES_IN_CBM_BYTE || pos+PULSES_IN_CBM_BYTE > len(tapData) {
		return -1, 0, fmt.Errorf("cbm_readbyte: position %d out of bounds", pos)
	}

	// Check next 20 pulses are not inside a pause and are inside the tap.
	for i := 0; i < PULSES_IN_CBM_BYTE; i++ {
		if pos+i >= len(tapData) { // Check bounds
			return -1, 0, fmt.Errorf("cbm_readbyte: position %d out of bounds during pause check", pos+i)
		}
		if tapData[pos+i] == 0 { // Check for pause (pulse length 0)
			return -1, 0, fmt.Errorf("cbm_readbyte: unexpected pause at pulse position %d", pos+i)
		}
	}

	var bit int
	var err error

	bit, err = cbm_readbit(tapData, pos, sp, mp, lp, tol)
	if err != nil || bit != 2 { // Must start with a new data marker
		return -1, 0, fmt.Errorf("no new data marker: %v", err)
	}

	byteVal := 0
	check := 1
	pulsesRead := 2 // Start from 2, as we've consumed the LM pair here


	for i := 0; i < 8; i++ {
		bit, err = cbm_readbit(tapData, pos+pulsesRead, sp, mp, lp, tol)
		if err != nil || (bit != 0 && bit != 1) {
			return -1, 0, fmt.Errorf("invalid data bit: %v", err)
		}
		byteVal |= bit << i
		check ^= bit
		pulsesRead += 2
	}

	bit, err = cbm_readbit(tapData, pos+pulsesRead, sp, mp, lp, tol)
	if err != nil || bit != check {
		return -1, 0, fmt.Errorf("checksum bit mismatch: %v", err)
	}
	pulsesRead += 2

	return byteVal, pulsesRead, nil
}

// IsCBMLead checks for the C64 CBM loader signature.
func IsCBMLead(tapData []byte, startPos int, defaultTolerance, pulsesInByte int) (bool, int, int, int, int) {
    cbmFmt := Ft[CBM_HEAD]
    sp, mp, lp, pmin := cbmFmt.SP, cbmFmt.MP, cbmFmt.LP, cbmFmt.PMin

	// CBM pilot is a sequence of short pulses
	count := 0
	currentPos := startPos
	// Scan for a continuous block of short pulses
	for currentPos < len(tapData) && (int(tapData[currentPos]) < sp-defaultTolerance || int(tapData[currentPos]) > sp+defaultTolerance) {
		currentPos++
	}

	pilotStart := currentPos
	for currentPos < len(tapData) && (int(tapData[currentPos]) >= sp-defaultTolerance && int(tapData[currentPos]) <= sp+defaultTolerance) {
		count++
		currentPos++
	}

	if count < pmin {
		return false, 0, 0, 0, 0
	}
	pilotEndPos := pilotStart + count

    currentPos = pilotEndPos

    // Check for sync sequence
    firstByte, pulsesRead, err := cbm_readbyte(tapData, currentPos, sp, mp, lp, defaultTolerance)
    if err != nil {
        return false, 0, 0, 0, 0
    }
    currentPos += pulsesRead

    isFirst := (firstByte == 0x09 || firstByte == 0x89)
    if !isFirst {
        return false, 0, 0, 0, 0
    }

    expected := firstByte - 1
    for i := 0; i < 8; i++ {
        val, pulses, err := cbm_readbyte(tapData, currentPos, sp, mp, lp, defaultTolerance)
        if err != nil || val != int(expected) {
            return false, 0, 0, 0, 0
        }
        currentPos += pulses
        expected--
    }

    // Read header info
    fileType, pulses, err := cbm_readbyte(tapData, currentPos, sp, mp, lp, defaultTolerance)
    if err != nil {
        return false, 0, 0, 0, 0
    }
    currentPos += pulses

    startAddrLow, pulses, err := cbm_readbyte(tapData, currentPos, sp, mp, lp, defaultTolerance)
    if err != nil { return false, 0, 0, 0, 0 }
    currentPos += pulses

    startAddrHigh, pulses, err := cbm_readbyte(tapData, currentPos, sp, mp, lp, defaultTolerance)
    if err != nil { return false, 0, 0, 0, 0 }
    currentPos += pulses

    endAddrLow, pulses, err := cbm_readbyte(tapData, currentPos, sp, mp, lp, defaultTolerance)
    if err != nil { return false, 0, 0, 0, 0 }
    currentPos += pulses

    endAddrHigh, pulses, err := cbm_readbyte(tapData, currentPos, sp, mp, lp, defaultTolerance)
    if err != nil { return false, 0, 0, 0, 0 }
    currentPos += pulses

    // Find end of data by looking for the end of data marker
    eod := currentPos
    for eod < len(tapData) -1 {
        bit, err := cbm_readbit(tapData, eod, sp, mp, lp, defaultTolerance)
        if err != nil || bit == 3 { // End of data marker or error
            break
        }
        eod += 2 // Move to the next pulse pair
    }

    // After the CBM header, check for a short pause and include it.
    // This is to handle cases where a pause is part of the header block.
    pausePos := eod
    for pausePos < len(tapData) && int(tapData[pausePos]) != 0 {
        pausePos++
    }
    if pausePos > eod && pausePos < len(tapData) {
        // Include the pause in the block
        eod = pausePos
    }

    startAddr := startAddrLow | (startAddrHigh << 8)
    endAddr := endAddrLow | (endAddrHigh << 8)
    psize := endAddr - startAddr

    // Check if it's a CBM HEADER block
    isHeader := (fileType >= 1 && fileType <= 5 && endAddr > startAddr)
    if !isHeader {
        return false, 0, 0, 0, 0
    }

    return true, eod - startPos, fileType, psize, pilotEndPos
}

// DecodeCBMHeader decodes the raw pulse data of a CBM header block into a slice of bytes.
func DecodeCBMHeader(pulseData []byte, headerStartPos int, sp, mp, lp, tol int) ([]byte, error) {
	var decodedBytes []byte // Dynamically sized
	currentPulsePos := headerStartPos

	for { // Loop indefinitely until break condition
		// Check if we've reached the end of the pulseData
		if currentPulsePos+PULSES_IN_CBM_BYTE > len(pulseData) {
			break // Reached end of data, stop decoding
		}

		// Read the next bit pair to check for end of data marker
		bitres, err := cbm_readbit(pulseData, currentPulsePos, sp, mp, lp, tol)
		if err != nil {
			// If there's an error reading the bit, it might be corrupted data or end of block.
			// For now, we'll break and return what we've decoded so far.
			break
		}

		if bitres == 3 { // LS (End of data) marker
			// This is the end of the header data. Stop decoding.
			break
		}

		// If it's not an LM (New data) marker, it's an unexpected bit type for a header byte.
		// This might indicate corrupted data or a non-standard header.
		// The C code's cbm_readbyte expects an LM marker.
		// So, if bitres is not 2, it's an error.
		if bitres != 2 {
			// For now, we'll break and return what we've decoded so far.
			break
		}

		// Now, read the actual byte
		byteVal, pulsesRead, err := cbm_readbyte(pulseData, currentPulsePos, sp, mp, lp, tol)
		if err != nil {
			return nil, fmt.Errorf("error decoding CBM header byte at pulse position %d: %w", currentPulsePos, err)
		}

		decodedBytes = append(decodedBytes, byte(byteVal))
		currentPulsePos += pulsesRead
	}
	return decodedBytes, nil
}

// IsCBMData checks for the C64 CBM data loader signature.
func IsCBMData(tapData []byte, startPos int, defaultTolerance, pulsesInByte int) (bool, int, int, int, int) {
    cbmFmt := Ft[CBM_DATA]
    sp, mp, lp, pmin := cbmFmt.SP, cbmFmt.MP, cbmFmt.LP, cbmFmt.PMin

	// CBM pilot is a sequence of short pulses
	count := 0
	currentPos := startPos
	// Scan for a continuous block of short pulses
	for currentPos < len(tapData) && (int(tapData[currentPos]) < sp-defaultTolerance || int(tapData[currentPos]) > sp+defaultTolerance) {
		currentPos++
	}

	pilotStart := currentPos
	for currentPos < len(tapData) && (int(tapData[currentPos]) >= sp-defaultTolerance && int(tapData[currentPos]) <= sp+defaultTolerance) {
		count++
		currentPos++
	}

	if count < pmin {
		return false, 0, 0, 0, 0
	}
	pilotEndPos := pilotStart + count

    currentPos = pilotEndPos

    // Check for sync sequence
    firstByte, pulsesRead, err := cbm_readbyte(tapData, currentPos, sp, mp, lp, defaultTolerance)
    if err != nil {
        return false, 0, 0, 0, 0
    }
    currentPos += pulsesRead

    isFirst := (firstByte == 0x09 || firstByte == 0x89)
    if !isFirst {
        return false, 0, 0, 0, 0
    }

    expected := firstByte - 1
    for i := 0; i < 8; i++ {
        val, pulses, err := cbm_readbyte(tapData, currentPos, sp, mp, lp, defaultTolerance)
        if err != nil || val != int(expected) {
            return false, 0, 0, 0, 0
        }
        currentPos += pulses
        expected--
    }

    // Read file type
    fileType, pulses, err := cbm_readbyte(tapData, currentPos, sp, mp, lp, defaultTolerance)
    if err != nil {
        return false, 0, 0, 0, 0
    }
    currentPos += pulses

    // Find end of data by looking for the end of data marker
    eod := currentPos
    for eod < len(tapData) -1 {
        bit, err := cbm_readbit(tapData, eod, sp, mp, lp, defaultTolerance)
        if err != nil || bit == 3 { // End of data marker or error
            break
        }
        eod += 2 // Move to the next pulse pair
    }

    // Check if it's a CBM DATA block (fileType 0 for PRG, 2 for SEQ)
    isData := (fileType == 0 || fileType == 2)
    if !isData {
        return false, 0, 0, 0, 0
    }

    return true, eod - startPos, fileType, 0, pilotEndPos // psize is not relevant for data blocks here
}
