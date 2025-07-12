// internal/idx/handler.go

// package idx handles reading .idx files. these contain hexadecimal byte offsets
// and corresponding block names for .tap files, used by emulators or hardware
// devices for quick program access on multi-load tapes. here they are used to
// provide meaningful labels (tags) for the corresponding audio/data blocks
// detected during the main .tap processing workflow (in package audio).
package idx

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	idxPositionBase = 16 // hexadecimal position
	idxPositionBits = 32 // assuming positions fit within 32 bits
)

// IDXEntry holds data parsed from one line of a .idx file.
// The Position field represents the byte offset within the associated .tap file.
// typically, this offset includes the standard .tap header length - however,
// variations exist. inconsistencies in position are handled during merging/tagging
// processes in package audio using a proximity tolerance.
type IDXEntry struct {
	Position int    // byte offset position within the associated .tap file
	Name     string // tag or name associated with this position
}

// ReadIDX opens and parses a tape index (.idx) file specified by filepath.
// it expects lines in the format "<HexPosition> <Name>", allowing an optional "0x"
// prefix for the position. comment lines starting with ';' and empty lines are
// skipped. returns a slice of IDXEntry structs containing the parsed positions and
// names - or an error if opening or parsing fails.
func ReadIDX(filepath string) ([]IDXEntry, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("error opening idx file %s: %w", filepath, err)
	}
	defer file.Close()

	var entries []IDXEntry // slice to hold results
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	// read file line by line
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		// expect format "<position> <name>", split at first space only
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			// return error indicating format issue and line number
			return nil, fmt.Errorf("line %d: invalid idx line format: %s", lineNumber, line)
		}

		// parse the position part (hexadecimal)
		positionStr := strings.TrimPrefix(parts[0], "0x") // allow optional "0x" prefix
		position, err := strconv.ParseInt(positionStr, idxPositionBase, idxPositionBits)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid hex position '%s': %w", lineNumber, parts[0], err)
		}

		// parse the name part (trim extra space)
		name := strings.TrimSpace(parts[1])

		// append valid entry to the slice
		entries = append(entries, IDXEntry{Position: int(position), Name: name})
	}

	// check for errors during scanning (e.g., i/o error)
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning idx file %s: %w", filepath, err)
	}

	// return successfully parsed entries
	return entries, nil
}
