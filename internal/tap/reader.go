// internal/tap/reader.go

// package tap provides functionality for reading and validating CBM TAP tape image files.
package tap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"go_chirp_the_tap/internal/constants"
	"io"
	"os"
)

// ReadTAP opens, validates, and reads the entire content of a .tap file (v0 or v1).
// it checks the file signature, version, minimum length and declared data size
// against the actual file size.
// on success, it returns the full byte content of the file (including the header).
func ReadTAP(filepath string) ([]byte, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("error opening tap file '%s': %w", filepath, err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading tap file '%s': %w", filepath, err)
	}

	// check minimum length: valid .tap files must be atleast as long as the size of a header...
	if len(data) < constants.TapHeaderSize {
		return nil, fmt.Errorf("invalid tap file '%s': file too short (%d bytes found, %d required)", filepath, len(data), constants.TapHeaderSize)
	}

	// check file for valid file signature
	signature := data[0 : 0+12]
	expectedSignature := []byte(constants.TapSignatureC64)
	if !bytes.Equal(signature, expectedSignature) {
		return nil, fmt.Errorf("invalid tap file '%s': incorrect signature (expected '%s', got '%s')", filepath, constants.TapSignatureC64, string(signature))
	}

	// check for supported .tap version
	version := data[12]
	if version > constants.TapMaxVersionSupport {
		return nil, fmt.Errorf("invalid tap file '%s': unsupported version %d (only versions <= %d supported)", filepath, version, constants.TapMaxVersionSupport)
	}

	// check declared data size against actual file data size
	// data size field in header = number of bytes after the 20-byte header
	expectedDataSize := binary.LittleEndian.Uint32(data[16 : 16+4])
	actualDataSize := uint32(len(data) - constants.TapHeaderSize) // actual number of bytes after header

	if actualDataSize != expectedDataSize {
		return nil, fmt.Errorf("invalid tap file '%s': declared data size (in header) (%d) does not match actual data size (%d)", filepath, expectedDataSize, actualDataSize)
	}

	// if checks pass, return file
	return data, nil
}
