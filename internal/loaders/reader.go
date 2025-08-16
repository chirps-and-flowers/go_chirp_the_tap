package loaders

import (
	"fmt"
	"go_chirp_the_tap/internal/constants"
)

// ReadBit reads a single pulse from tapData at 'pos' and interprets it as a bit (0 or 1).
// This is a more faithful port of the logic from tapclean's readttbit function.
func ReadBit(tapData []byte, pos int, lp, sp, tp, tol int) (int, error) {
	if pos < 0 || pos >= len(tapData) {
		return -1, fmt.Errorf("readBit: position %d out of bounds (tapData length %d)", pos, len(tapData))
	}

	p1 := int(tapData[pos])

	if p1 < sp-tol {
		return -1, fmt.Errorf("readBit: pulse at pos %d (val %d) is too short (sp:%d, tol:%d)", pos, p1, sp, tol)
	}
	if p1 > lp+tol {
		return -1, fmt.Errorf("readBit: pulse at pos %d (val %d) is too long (lp:%d, tol:%d)", pos, p1, lp, tol)
	}

	if tp != NA {
		if p1 < tp {
			return 0, nil // Short pulse
		} else {
			return 1, nil // Long pulse
		}
	} else {
		// Midpoint method
		if abs(p1-sp) < abs(p1-lp) {
			return 0, nil // Closer to short pulse
		} else {
			return 1, nil // Closer to long pulse
		}
	}
}

// ReadByte reads 8 bits using ReadBit and assembles them into a byte, respecting endianness.
// Returns the byte value on success, or an error on failure.
func ReadByte(tapData []byte, pos int, lp, sp, tp, endi, tol int) (int, error) {
	var b int

	// Check if there are enough pulses for a byte
	if pos < 0 || pos+7 >= len(tapData) {
		return -1, fmt.Errorf("readByte: not enough pulses for a byte at position %d (tapData length %d)", pos, len(tapData))
	}

	for i := 0; i < 8; i++ {
		bit, err := ReadBit(tapData, pos+i, lp, sp, tp, tol)
		if err != nil {
			return -1, fmt.Errorf("readByte: failed to read bit %d at position %d: %w", i, pos+i, err)
		}

		if endi == MSbF {
			b |= bit << (7 - i)
		} else { // LSbF
			b |= bit << i
		}
	}

	return b, nil
}

// findPilot searches for a pilot/sync sequence at 'pos' in tapData based on the format 'fmtIdx'.
// Returns the end position of the pilot, a boolean indicating if a legal quantity was found,
// and an error if a critical reading error occurs.
func findPilot(tapData []byte, pos int, fmtIdx int, tol int) (int, bool, error) {
	if pos < constants.TapHeaderSize {
		return 0, false, nil // Not enough data for a pilot at the very beginning
	}

	// Retrieve format parameters
	fmt := Ft[fmtIdx]
	sp := fmt.SP
	lp := fmt.LP
	tp := fmt.TP
	en := fmt.Endian
	pv := fmt.PV
	sv := fmt.SV
	pmin := fmt.PMin
	pmax := fmt.PMax

	if pmax == NA {
		pmax = 200000 // set some crazy limit if pmax is NA
	}

	var z int // Counter for pilot bits/bytes
	currentPos := pos

	// Check if pilot/sync values are BIT values (0 or 1)
	if (pv == 0 || pv == 1) && (sv == 0 || sv == 1) { // are the pilot/sync BIT values?...
		bit, err := ReadBit(tapData, currentPos, lp, sp, tp, tol)
		if err != nil {
			return 0, false, err // Critical error reading bit
		}

		if bit == pv { // got pilot bit?
			z = 0
			for currentPos < len(tapData) {
				bit, err := ReadBit(tapData, currentPos, lp, sp, tp, tol)
				if err != nil {
					// If we encounter an error during pilot reading, we stop and return what we have.
					// tapclean's find_pilot returns 0 or negative pos on error, but doesn't propagate error.
					// For now, we'll return the error.
					break
				}
				if bit == pv {
					z++
					currentPos++ // Assuming 1 pulse per bit for iteration
				} else {
					break // Non-pilot bit found
				}
			}

			if z == 0 {
				return 0, false, nil // No pilot found
			}

			if z < pmin || z > pmax { // too few/many pilots?
				return currentPos, false, nil // Return end position, but indicate illegal quantity
			}

			return currentPos, true, nil // Enough pilots, return end position and true
		}
	} else { // Pilot/sync are BYTE values...
		byteVal, err := ReadByte(tapData, currentPos, lp, sp, tp, en, tol)
		if err != nil {
			return 0, false, err // Critical error reading byte
		}

		if byteVal == pv { // got pilot byte?
			z = 0
			for currentPos < len(tapData) {
				byteVal, err := ReadByte(tapData, currentPos, lp, sp, tp, en, tol)
				if err != nil {
					// Similar to bit reading, stop and return error if encountered.
					break
				}
				if byteVal == pv {
					z++
					currentPos += 8 // Assuming 8 pulses per byte for iteration
				} else {
					break // Non-pilot byte found
				}
			}

			if z == 0 {
				return 0, false, nil // No pilot found
			}

			if z < pmin || z > pmax { // too few/many pilots?
				return currentPos, false, nil // Return end position, but indicate illegal quantity
			}

			return currentPos, true, nil // Enough pilots, return end position and true
		}
	}

	return 0, false, nil // No pilot found at starting position
}

// abs returns the absolute value of an integer.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
