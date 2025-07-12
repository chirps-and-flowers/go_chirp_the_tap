// cmd/main.go

// command go_chirp_the_tap converts cbm tap (digitized tape files) files into audio formats or cpk packages.
package main

import (
	"flag"
	"fmt"
	"go_chirp_the_tap/internal/audio"
	"go_chirp_the_tap/internal/constants"
	"go_chirp_the_tap/internal/export"
	"go_chirp_the_tap/internal/idx"
	"go_chirp_the_tap/internal/tap"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type OutputFormat string

const (
	FormatWAV OutputFormat = "wav"
	FormatPCM OutputFormat = "pcm"
)

func main() {
	// main entry point for the go_chirp_the_tap command-line tool.
	// workflow summary:
	// 1. parse flags (-format, -cpk, -csv, -clock) & get input .tap file path.
	// 2. prepare output paths & select clock frequency (pal/ntsc).
	// 3. read .tap file and optional associated .idx file.
	// 4. call audio.processtapdata to get pcm samples & detailed segment index (indexData).
	// 5. generate final output (.cpk package or .wav/.pcm + optional .csv) based on flags.
	// exits via log.fatal on critical errors.

	// command-line arguments
	format := flag.String("format", string(FormatWAV), "Output format (wav or pcm)")
	cpk := flag.Bool("cpk", false, "Create a cpk-package (.cpk archive with wav blocks and csv)")
	csv := flag.Bool("csv", false, "Generate standalone CSV file (only if --cpk is not set)")
	clockType := flag.String("clock", "pal", "Clock speed standard ('pal' or 'ntsc')")
	targetSystem := flag.String("target", "c64", "Target system (e.g., c64, amstrad, spectrum)")
	flag.Parse() // parse command-line arguments into defined flags

	// access flag values and non-flag args below this point
	args := flag.Args()
	if len(args) < 1 {
		log.Fatal("error: please provide a tap file path as an argument")
	}
	tapFilePath := args[0]
	fmt.Printf("Input TAP file: %s\n", tapFilePath)

	// prep output path, name and extension
	outputExt := filepath.Ext(tapFilePath)
	baseFilePath := tapFilePath[:len(tapFilePath)-len(outputExt)]
	outputFormat := OutputFormat(*format)
	var outputAudioPath string
	switch outputFormat {
	case FormatWAV:
		outputAudioPath = baseFilePath + ".wav"
	case FormatPCM:
		outputAudioPath = baseFilePath + ".pcm"
	default:
		log.Fatalf("Error: unsupported output format: %s. Use 'wav' or 'pcm'.", *format)
	}
	outputCSVPath := baseFilePath + ".csv"
	idxFilePath := baseFilePath + ".idx"
	cpkPackagePath := baseFilePath + ".cpk"

	// get clock speed based on flag value
	selectedClock, err := selectClock(*clockType)
	if err != nil {
		log.Fatalf("Error selecting clock: %v", err)
	}

	// declare vars for holding tap/idx data and processing results
	var tapPayload []byte            // holds raw data blocks read from the .tap file
	var tapVersion byte              // holds the version byte read from the .tap header
	var idxEntries []idx.IDXEntry    // holds entries read from the optional .idx file (nil if no file)
	var pcmSamples []byte            // holds the generated raw pcm audio sample data
	var indexData []audio.IndexEntry // holds index metadata generated during audio processing

	// read .tap file
	fmt.Printf("Reading TAP file: %s\n", tapFilePath)
	tapData, err := tap.ReadTAP(tapFilePath)
	if err != nil {
		log.Fatalf("Error reading TAP file: %v", err)
	}

	// ensure file is large enough to contain the expected header
	if len(tapData) < constants.TapHeaderSize {
		log.Fatalf("Invalid TAP file: shorter than header size (%d bytes)", constants.TapHeaderSize)
	}
	tapVersion = tapData[12] // offset 12 holds the version byte in cbm tap header v0/v1

	tapPayload = tapData[constants.TapHeaderSize:]
	fmt.Printf("TAP version: %d, Payload size: %d bytes\n", tapVersion, len(tapPayload))

	// read .idx file
	if _, err := os.Stat(idxFilePath); err == nil {
		idxEntries, err = idx.ReadIDX(idxFilePath)
		if err != nil {
			// idx read error treated as non-fatal - we  just proceed without .idx metadata
			log.Printf("Warning: Error reading IDX file '%s': %v...\n", idxFilePath, err)
			idxEntries = nil
		} else {
			fmt.Printf("Read %d entries from IDX file: %s\n", len(idxEntries), idxFilePath)
		}
	} else if !os.IsNotExist(err) {
		log.Printf("Warning: Error checking for IDX file '%s': %v...\n", idxFilePath, err)
	} else {
		fmt.Println("No IDX file found. Processing without IDX data.")
	}

	// process .tap (and .idx if available)
	fmt.Println("Processing TAP data into audio...")

	pcmSamples, indexData, err = audio.ProcessTAPData(tapData, tapVersion, selectedClock, constants.SampleRate, idxEntries)
	if err != nil {
		log.Fatalf("Error processing TAP data: %v", err)
	}
	fmt.Printf("Generated %d PCM samples. Found %d raw index entries.\n", len(pcmSamples), len(indexData))

	// generate output
	if *cpk {
		fmt.Printf("Creating cpk package: %s\n", cpkPackagePath)

		err = export.SplitAndPackageBlocks(pcmSamples, indexData, baseFilePath, int(constants.SampleRate), selectedClock, *targetSystem)
		if err != nil {
			log.Fatalf("Error creating cpk package: %v", err)
		}
		fmt.Printf("CPK package created successfully.\n")
	} else {
		fmt.Printf("Writing audio file: %s (Format: %s)\n", outputAudioPath, outputFormat)

		switch outputFormat {
		case FormatWAV:
			err = audio.WriteWAVFile(outputAudioPath, pcmSamples, int(constants.SampleRate))
		case FormatPCM:
			err = os.WriteFile(outputAudioPath, pcmSamples, 0644)
		}
		if err != nil {
			log.Fatalf("Error writing audio file '%s': %v", outputAudioPath, err)
		}
		fmt.Printf("Audio file written successfully.\n")

		if *csv {
			fmt.Printf("Writing CSV file: %s\n", outputCSVPath)

			_, err = export.ExportBlockInfo(indexData, outputCSVPath, constants.SampleRate)
			if err != nil {
				log.Fatalf("Error writing CSV file '%s': %v", outputCSVPath, err)
			}
			fmt.Printf("CSV file written successfully.\n")
		} else {
			fmt.Println("Standalone CSV file generation not requested (--csv flag not set).")
		}
	}

	fmt.Println("Processing finished.")
}

// helper for pal/ntsc clock argument selector
func selectClock(clockType string) (float64, error) {
	switch strings.ToLower(clockType) {
	case "ntsc":
		fmt.Println("Using NTSC clock.")
		return constants.ClockNTSC, nil
	case "pal":
		fmt.Println("Using PAL clock.")
		return constants.ClockPAL, nil
	default:
		return 0, fmt.Errorf("invalid clock type '%s' (must be 'pal' or 'ntsc')", clockType)
	}
}
