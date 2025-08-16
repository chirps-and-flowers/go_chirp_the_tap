package tap

import (
	"bytes"
	"fmt"
	"go_chirp_the_tap/internal/audio"
	"go_chirp_the_tap/internal/constants"
	"go_chirp_the_tap/internal/loaders"
	"strings"
)

// HeaderInfo contains information extracted from a C64 tape header
type HeaderInfo struct {
	FileName     string // Game name in ASCII
	LoadAddress  uint16 // Memory address where file should be loaded
	EndAddress   uint16 // Memory address where file ends
	FileType     string // Type of file (CBM HEADER, TURBOTAPE HEADER, etc.)
	BlockID      int    // Block identifier
}

// ReadCBMHeader extracts information from a CBM header block
func ReadCBMHeader(block *audio.IndexEntry) (*HeaderInfo, error) {
	if block == nil {
		return nil, fmt.Errorf("block is nil")
	}


	// Validate block type
	if block.Type != "cbm_head" {
		return nil, fmt.Errorf("block is not a CBM header")
	}

	// CBM headers are 192 bytes with specific structure
	// Bytes 5-20 contain the filename (16 bytes, PETSCII encoded)
	// Bytes 1-2 contain load address (little endian)
	// Bytes 3-4 contain end address (little endian)
	// Byte 0 contains file type

	// Get CBM format parameters
	cbmFmt := loaders.Ft[loaders.CBM_HEAD]
	sp, mp, lp := cbmFmt.SP, cbmFmt.MP, cbmFmt.LP

	// Calculate the actual start of the header within block.RawData
	// block.StartPosition is the start of the block in the original tapData.
	// block.PilotEndPos is the end of the pilot in the original tapData.
	// So, the offset of pilotEndPos within block.RawData is block.PilotEndPos - block.StartPosition.
	// The sync sequence is 10 bytes * 20 pulses/byte = 200 pulses.
	// So, the header starts at (block.PilotEndPos - block.StartPosition) + 200 pulses within block.RawData.

	headerStartOffsetInRawData := (block.PilotEndPos - block.StartPosition) + (9 * loaders.PULSES_IN_CBM_BYTE)

	decodedHeader, err := loaders.DecodeCBMHeader(block.RawData, headerStartOffsetInRawData, sp, mp, lp, constants.DefaultTolerance)
	
	if err != nil {
		return nil, fmt.Errorf("error decoding CBM header: %w", err)
	}

	

	headerInfo := &HeaderInfo{
		FileType: "CBM HEADER",
		BlockID:  block.BlockID,
	}

	// Extract filename (16 bytes starting at offset 5)
	filenameBytes := make([]byte, 16)
	copy(filenameBytes, decodedHeader[5:21])

	// Trim null bytes from the end of the filenameBytes before converting to text
	trimmedFilenameBytes := bytes.TrimRight(filenameBytes, "\x00")

	// Convert PETSCII to readable text
	filename := pet2text(trimmedFilenameBytes)
	headerInfo.FileName = strings.TrimSpace(filename)

	// Extract load address (bytes 1-2, little endian)
	headerInfo.LoadAddress = uint16(decodedHeader[1]) | (uint16(decodedHeader[2]) << 8)

	// Extract end address (bytes 3-4, little endian)
	headerInfo.EndAddress = uint16(decodedHeader[3]) | (uint16(decodedHeader[4]) << 8)

	
	return headerInfo, nil
}

// ReadTurboTape250Header extracts information from a Turbo Tape 250 header block
func ReadTurboTape250Header(block *audio.IndexEntry) (*HeaderInfo, error) {
	if block == nil {
		return nil, fmt.Errorf("block is nil")
	}

	// Get Turbotape format parameters
	turbotapeFmt := loaders.Ft[loaders.TT_HEAD] // Use TT_HEAD for parameters
	lp, sp, tp := turbotapeFmt.LP, turbotapeFmt.SP, turbotapeFmt.TP
	en := turbotapeFmt.Endian // MSbF for Turbotape

	// Calculate the actual start of the header within block.RawData
	headerStartOffsetInRawData := (block.PilotEndPos - block.StartPosition) + (9 * 8) // 9 sync bytes * 8 pulses/byte

	// Decode the raw pulse data into bytes
	decodedHeader, err := loaders.DecodeTurbotapeHeader(block.RawData, headerStartOffsetInRawData, lp, sp, tp, en, constants.DefaultTolerance)
	if err != nil {
        return nil, fmt.Errorf("error decoding Turbo Tape 250 header: %w", err)
    }

	// Ensure enough data is decoded for the header structure
	// Minimum valid header is ID(1) + Start(2) + End(2) + Filename(1+) = 6 bytes
	const MIN_HEADER_LEN = 6
	if len(decodedHeader) < MIN_HEADER_LEN {
		return nil, fmt.Errorf("insufficient decoded data for Turbo Tape 250 header")
	}

	headerInfo := &HeaderInfo{
		FileType: "TURBOTAPE HEADER", // Default, will be adjusted by caller
		BlockID:  block.BlockID,
	}

	// Extract filename (up to 16 bytes starting at offset 6 in the decoded header)
	filenameBytes := make([]byte, 16)
	end := 6 + 16
	if end > len(decodedHeader) {
		end = len(decodedHeader)
	}
	copy(filenameBytes, decodedHeader[6:end])

	filename := pet2text(filenameBytes)
	headerInfo.FileName = strings.TrimRight(strings.TrimSpace(filename), "\x00")

	// Extract load address (bytes 1-2 in the decoded header)
	headerInfo.LoadAddress = uint16(decodedHeader[1]) | (uint16(decodedHeader[2]) << 8)

	// Extract end address (bytes 3-4 in the decoded header)
	headerInfo.EndAddress = uint16(decodedHeader[3]) | (uint16(decodedHeader[4]) << 8)

	return headerInfo, nil
}


// pet2text converts PETSCII encoded bytes to ASCII text
func pet2text(petscii []byte) string {
	result := make([]byte, len(petscii))
	for i := 0; i < len(petscii); i++ {
		b := petscii[i]
		
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
			// For other characters, just pass them through for now
			result[i] = b
		}
	}
	
	return string(result)
}

// ExtractHeadersFromTAP processes a TAP file and extracts all recognizable headers
func ExtractHeadersFromTAP(blocks []*audio.IndexEntry) ([]*HeaderInfo, error) {
	var headers []*HeaderInfo
	
	for _, block := range blocks {
		var header *HeaderInfo
		var err error
		
		// Process CBM headers
		if block.Type == "cbm_head" {
			header, err = ReadCBMHeader(block)
			if err == nil {
				headers = append(headers, header)
			}
		}
		
		// Process Turbo Tape 250 headers
		if block.Type == "tt_head" {
			header, err = ReadTurboTape250Header(block)
			if err == nil {
				headers = append(headers, header)
			}
		}
	}
	
	return headers, nil
}

// GetGameNamesFromHeaders extracts just the game names from header information
func GetGameNamesFromHeaders(headers []*HeaderInfo) []string {
	var names []string
	
	for _, header := range headers {
		if header.FileName != "" {
			names = append(names, header.FileName)
		}
	}
	
	return names
}

// ExtractAndApplyHeaders iterates through blocks, extracts header info, and applies the filename to the IDXTag field.
func ExtractAndApplyHeaders(blocks []audio.IndexEntry) {
	
	for i := range blocks {
		block := &blocks[i]
		var header *HeaderInfo
		var err error

		switch block.Type {
		case "cbm_head":
			header, err = ReadCBMHeader(block)
			
		case "tt_head":
            // Only process if it's a Turbotape header (BlockID 1 or 2)
            if block.BlockID == 1 || block.BlockID == 2 {
                header, err = ReadTurboTape250Header(block)
                if err == nil && header != nil {
                    header.FileType = "TURBOTAPE HEADER"
                    
                }
            }
		
		}

		if err == nil && header != nil {
			block.IDXTag = header.FileName
		}
	}
}
