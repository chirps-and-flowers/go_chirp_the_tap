// internal/export/chirp_package.go

package export

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"go_chirp_the_tap/internal/audio"
	"go_chirp_the_tap/internal/constants"
	"os"
	"path/filepath" // needed for manifest (base)
	"time"          // needed for manifest timestamp
)

// PackageManifest defines the structure for the package_manifest.json file
// included within the .cpk archive.
type PackageManifest struct {
	TargetSystem       string  `json:"target_system"`         // placeholder
	ClockStandard      string  `json:"clock_standard"`        // "pal", "ntsc", or "unknown"
	ClockFrequency     float64 `json:"clock_frequency"`       // cpu clock frequency in hz used for processing
	SampleRate         int     `json:"sample_rate"`           // audio sample rate in hz
	SourceFile         string  `json:"source_file"`           // base name of the original .tap file
	Polarity           string  `json:"polarity"`              // signal polarity used
	Waveform           string  `json:"waveform"`              // waveform used for pulses (only square atm)
	AudioBitsPerSample int     `json:"audio_bits_per_sample"` // bits per audio sample (e.g., 8)
	AudioChannels      int     `json:"audio_channels"`        // number of audio channels (e.g., 1 for mono)
	CreationTimestamp  string  `json:"creation_timestamp"`    // timestamp when the cpk file was created
}

// SplitAndPackageBlocks generates a .cpk archive (gzipped tarball).
// the archive contains a manifest file (package_manifest.json), a block index (blocks.csv),
// and individual audio blocks as separate .wav files based on the provided indexData.
func SplitAndPackageBlocks(pcmSamples []byte, indexData []audio.IndexEntry, baseFilePath string, sampleRate int, selectedClock float64, targetSystem string) (err error) {
	if sampleRate <= 0 {
		return fmt.Errorf("invalid sample rate: %d", sampleRate)
	}
	floatSampleRate := float64(sampleRate)

	outPath := baseFilePath + ".cpk"
	file, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("error creating output file %s: %w", outPath, err)
	}
	// setup defer for closing file, check error later using named return 'err'
	defer func() {
		closeErr := file.Close()
		if err == nil && closeErr != nil { // only overwrite err if no previous error occurred
			err = fmt.Errorf("error closing output file %s: %w", outPath, closeErr)
		} else if closeErr != nil {
			// log error if another error already occurred
			fmt.Printf("warning: error closing output file %s (previous error: %v): %v\n", outPath, err, closeErr)
		}
	}()

	// setup gzip and tar writers - note file compression is set to 7 for high compression and bearable speed
	gzWriter, err := gzip.NewWriterLevel(file, 7)
	if err != nil {
		return fmt.Errorf("error creating gzip writer: %w", err)
	}
	tarWriter := tar.NewWriter(gzWriter)

	// ensure writers are closed in reverse order via defer; capture potential errors
	defer func() {
		tarCloseErr := tarWriter.Close()
		if err == nil && tarCloseErr != nil {
			err = fmt.Errorf("error closing tar writer: %w", tarCloseErr)
		} else if tarCloseErr != nil {
			fmt.Printf("warning: error closing tar writer (previous error: %v): %v\n", err, tarCloseErr)
		}

		gzCloseErr := gzWriter.Close()
		if err == nil && gzCloseErr != nil {
			err = fmt.Errorf("error closing gzip writer: %w", gzCloseErr)
		} else if gzCloseErr != nil {
			fmt.Printf("warning: error closing gzip writer (previous error: %v): %v\n", err, gzCloseErr)
		}
	}()

	// generate and write manifest file (package_manifest.json)
	fmt.Println("creating manifest data...")
	manifest := PackageManifest{
		TargetSystem:       targetSystem,
		ClockFrequency:     selectedClock,
		SampleRate:         sampleRate,
		SourceFile:         filepath.Base(baseFilePath + ".tap"),
		Polarity:           "normal", // hardcoded assumption for now...
		Waveform:           "square",
		AudioBitsPerSample: 8,
		AudioChannels:      1,
		CreationTimestamp:  time.Now().UTC().Format(time.RFC3339),
	}

	// determine clock standard string ("PAL" or "NTSC") based on exact frequency value.
	if selectedClock == constants.ClockPAL {
		manifest.ClockStandard = constants.ClockStandardPAL
	} else if selectedClock == constants.ClockNTSC {
		manifest.ClockStandard = constants.ClockStandardNTSC
	} else {
		// should not be reacheable due to input validation in main func - included as a safeguard.
		manifest.ClockStandard = constants.ClockStandardUnknown
		fmt.Printf("warning: unexpected clock frequency %f processed; setting standard to unknown.\n", selectedClock)
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ") // pretty json
	if err != nil {
		return fmt.Errorf("error marshaling manifest to json: %w", err)
	}
	// write manifest to tar archive
	manifestHeader := &tar.Header{Name: "package_manifest.json", Size: int64(len(manifestData)), Mode: 0644, ModTime: time.Now()}
	if err = tarWriter.WriteHeader(manifestHeader); err != nil { // assign to existing err
		return fmt.Errorf("error writing manifest tar header: %w", err)
	}
	if _, err = tarWriter.Write(manifestData); err != nil { // assign to existing err
		return fmt.Errorf("error writing manifest json to tar: %w", err)
	}
	fmt.Println("manifest data written to archive.")

	// generate csv data in memory (blocks.csv)
	// passing "" as path and true for in-memory generation indicates it's for the archive
	fmt.Println("creating csv data...")
	csvData, err := ExportBlockInfo(indexData, "", floatSampleRate)
	if err != nil {
		return fmt.Errorf("error generating csv data for package: %w", err)
	}

	// process index entries and write individual wav blocks to tar archive
	fmt.Printf("processing %d index entries to create audio blocks...\n", len(indexData))
	blockCount := 0
	processedEntries := 0
	i := 0
	for i < len(indexData) {
		// periodic status update while processing files
		if processedEntries > 0 && processedEntries%20 == 0 {
			fmt.Printf("processed ~%d/%d index entries...\n", processedEntries, len(indexData))
		}

		// analyze current index entry(ies) to identify next logical block
		groupInfo := _getGroupedBlockInfo(indexData, i, floatSampleRate)

		// if a valid exportable block was identified by the analyzer...
		if groupInfo.IsBlock {
			// format filename like block_000_lead.wav, block_001_data.wav etc.
			wavFileName := fmt.Sprintf("block_%03d_%s.wav", blockCount, groupInfo.BlockType)
			blockStartSample := groupInfo.StartEntry.StartSample
			// add +1 to EndSample because slice range notation [start:end] is exclusive at the 'end' index
			blockEndSampleIndex := groupInfo.EndEntry.EndSample + 1

			// basic checks for sample indices
			if blockStartSample >= 0 && blockEndSampleIndex > blockStartSample {
				// ensure indices are within the bounds of the source pcmSamples slice
				if blockStartSample >= len(pcmSamples) {
					fmt.Printf("warning: block %d start sample %d out of bounds (pcm len %d), skipping.\n", blockCount, blockStartSample, len(pcmSamples))
					i += groupInfo.ConsumedEntries
					processedEntries += groupInfo.ConsumedEntries
					continue // continue to next iteration of outer loop
				}
				// cap end index if it goes beyond available pcm data (e.g., due to rounding)
				if blockEndSampleIndex > len(pcmSamples) {
					fmt.Printf("warning: block %d end sample %d out of bounds (pcm len %d), truncating.\n", blockCount, groupInfo.EndEntry.EndSample, len(pcmSamples))
					blockEndSampleIndex = len(pcmSamples)
				}

				// extract the pcm data slice for this block
				blockData := pcmSamples[blockStartSample:blockEndSampleIndex]

				// skip writing if the extracted block data is empty
				if len(blockData) == 0 {
					fmt.Printf("warning: block %d (%s) resulted in zero samples after slicing, skipping.\n", blockCount, wavFileName)
					i += groupInfo.ConsumedEntries
					processedEntries += groupInfo.ConsumedEntries
					continue // continue to next iteration of outer loop
				}

				// write this block as a separate wav file into the tar archive
				wavBuffer := new(bytes.Buffer) // use in-memory buffer to build wav file first
				// write header to buffer
				if err = audio.WriteWAVHeader(wavBuffer, sampleRate, len(blockData)); err != nil { // assign to existing err
					return fmt.Errorf("error writing wav header for %s: %w", wavFileName, err)
				}
				// write pcm data to buffer
				if _, err = wavBuffer.Write(blockData); err != nil { // assign to existing err
					return fmt.Errorf("error writing wav data for %s: %w", wavFileName, err)
				}
				// write buffer content to tar archive
				tarHeader := &tar.Header{Name: wavFileName, Size: int64(wavBuffer.Len()), Mode: 0644, ModTime: time.Now()}
				if err = tarWriter.WriteHeader(tarHeader); err != nil { // assign to existing err
					return fmt.Errorf("error writing tar header for %s: %w", wavFileName, err)
				}
				if _, err = tarWriter.Write(wavBuffer.Bytes()); err != nil { // assign to existing err
					return fmt.Errorf("error writing wav data to tar for %s: %w", wavFileName, err)
				}
				blockCount++ // increment successful block count

			} else {
				// log if sample range derived from groupInfo was invalid
				fmt.Printf("warning: invalid sample range for identified block starting near index %d (%s): start=%d, end=%d. skipping.\n", i, groupInfo.BlockType, blockStartSample, groupInfo.EndEntry.EndSample)
			}
		} // end if groupInfo.IsBlock

		// advance main loop index by number of entries consumed by the analyzer (1 or 2)
		i += groupInfo.ConsumedEntries
		processedEntries += groupInfo.ConsumedEntries // track for progress printing
	} // end wav block loop

	// write generated csv data to the tar archive (blocks.csv)
	fmt.Println("writing csv data to archive...")
	csvHeader := &tar.Header{Name: "blocks.csv", Size: int64(len(csvData)), Mode: 0644, ModTime: time.Now()}
	if err = tarWriter.WriteHeader(csvHeader); err != nil { // assign to existing err
		return fmt.Errorf("error writing csv tar header: %w", err)
	}
	if _, err = tarWriter.Write(csvData); err != nil { // assign to existing err
		return fmt.Errorf("error writing csv to tar: %w", err)
	}

	fmt.Printf("created archive with %d blocks, manifest, and csv: %s\n", blockCount, outPath)
	// note: defer handles closing writers and file; errors captured by named return 'err'
	return err // return the first error encountered during processing or closing (or nil if success)
}

// note: _getGroupedBlockInfo (from block_analyzer.go) and ExportBlockInfo from csv.go
