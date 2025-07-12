// mobile/api.go

package mobile

import (
	"errors"
	"fmt"
	"go_chirp_the_tap/internal/audio"
	"go_chirp_the_tap/internal/constants"
	"go_chirp_the_tap/internal/export"
	"go_chirp_the_tap/internal/idx"
	"go_chirp_the_tap/internal/tap"
	"os"
	"path/filepath"
	"strings"
)

// TestExport is a simple function to verify the mobile library is linked correctly.
func TestExport() string {
	return "hello from go"
}

// ProcessTAP2Pack creates a .cpk package from a .tap file.
// this is the main entry point for the mobile frontend. it handles file i/o,
// processes the raw tape data into audio samples, and packages the output.
//
// parameters:
//   - tapFilePath: absolute path to the source .tap file.
//   - clockType: clock standard to use ("pal" or "ntsc").
//   - targetSystem: target computer system (e.g., "c64").
//
// returns:
//   - string: the absolute path to the new .cpk file on success.
//   - error: an error if any part of the process fails.
func ProcessTAP2Pack(tapFilePath string, clockType string, targetSystem string) (string, error) {
	// construct paths based on the input file.
	baseFilePath := tapFilePath[:len(tapFilePath)-len(filepath.Ext(tapFilePath))]
	outputPackPath := baseFilePath + ".cpk"
	idxFilePath := baseFilePath + ".idx"

	// read the raw .tap file.
	tapData, err := tap.ReadTAP(tapFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to read tap file %s: %w", tapFilePath, err)
	}

	// validate header and get version.
	if len(tapData) < constants.TapHeaderSize {
		return "", fmt.Errorf("invalid tap file %s, smaller than header size", tapFilePath)
	}
	tapVersion := tapData[12]

	// optionally read the .idx file if it exists.
	var idxEntries []idx.IDXEntry
	if _, err := os.Stat(idxFilePath); err == nil {
		idxEntries, err = idx.ReadIDX(idxFilePath)
		if err != nil {
			return "", fmt.Errorf("failed to parse idx file %s: %w", idxFilePath, err)
		}
	} else if !os.IsNotExist(err) {
		// log a warning if we can't check for the file, but don't fail.
		fmt.Printf("warning: could not stat optional idx file %s: %v", idxFilePath, err)
	}

	// select the correct clock frequency.
	var clock float64
	switch strings.ToLower(clockType) {
	case "ntsc":
		clock = constants.ClockNTSC
	case "pal":
		clock = constants.ClockPAL
	default:
		return "", fmt.Errorf("invalid clock type '%s'", clockType)
	}

	// process the tape data into pcm samples and a block index.
	pcmSamples, indexData, err := audio.ProcessTAPData(tapData, tapVersion, clock, constants.SampleRate, idxEntries)
	if err != nil {
		return "", fmt.Errorf("failed to process tap data: %w", err)
	}
	if len(pcmSamples) == 0 {
		return "", errors.New("processing resulted in no audio samples")
	}

	// create the final .cpk package.
	err = export.SplitAndPackageBlocks(pcmSamples, indexData, baseFilePath, int(constants.SampleRate), clock, targetSystem)
	if err != nil {
		return "", fmt.Errorf("failed to create cpk package: %w", err)
	}

	// return the path to the new package.
	return outputPackPath, nil
}
