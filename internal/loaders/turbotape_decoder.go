package loaders

import (
	"fmt"
)

// DecodeTurbotapeHeader decodes the raw pulse data of a Turbotape header block into a slice of bytes.
// It reads as many full bytes as possible from the pulseData.
func DecodeTurbotapeHeader(pulseData []byte, headerStartPos int, lp, sp, tp, en, tol int) ([]byte, error) {
	var decodedBytes []byte
	currentPulsePos := headerStartPos

	for currentPulsePos+8 <= len(pulseData) { // Ensure there are enough pulses for a full byte
		byteVal, err := ReadByte(pulseData, currentPulsePos, lp, sp, tp, en, tol)
		if err != nil {
			// Stop if we can't read a full byte, which can happen at the end of the block.
			break
		}
		decodedBytes = append(decodedBytes, byte(byteVal))
		currentPulsePos += 8
	}

	if len(decodedBytes) == 0 {
		return nil, fmt.Errorf("no bytes could be decoded from turbotape header at pulse position %d", headerStartPos)
	}

	return decodedBytes, nil
}
