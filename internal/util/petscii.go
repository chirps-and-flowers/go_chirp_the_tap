package util

// Pet2Text converts PETSCII encoded bytes to ASCII string.
func Pet2Text(petscii []byte) string {
	result := make([]byte, len(petscii))
	for i, b := range petscii {
		// Process CHR$ 'SAME AS' codes...
		if b == 255 {
			b = 126
		} else if b > 223 && b < 255 { // produces 160-190
			b -= 64
		} else if b > 191 && b < 224 { // produces 96-127
			b -= 96
		}

		// Handle common PETSCII to ASCII conversions
		switch {
		case b >= 0x20 && b <= 0x7E: // ASCII printable characters
			result[i] = b
		case b == 0xA0: // Shifted space
			result[i] = 0x20
		default:
			// For other characters, use a period as placeholder
			result[i] = 0x2E
		}
	}
	return string(result)
}