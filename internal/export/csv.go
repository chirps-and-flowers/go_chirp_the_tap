// internal/export/csv.go

package export

import (
	"bytes"
	"fmt"
	"go_chirp_the_tap/internal/audio"
	"os"
	"strings"
	"text/tabwriter"
)

// ExportBlockInfo generates a formatted, human-readable .csv table consisting of
// block information primarily for use to be packaged and used by a frontend.
// It uses the _getGroupedBlockInfo helper to identify blocks.
//
// if an outputPath is provided, the function has the side effect of writing the
// generated data to that file path.
//
// parameters:
//   - indexData: A slice of audio.IndexEntry structs containing the metadata for each block.
//   - outputPath: The file path to write the .csv to. If this string is empty, the function
//     will not write to disk.
//   - sampleRate: The audio sample rate, required for accurately calculating block end times.
//
// returns:
//   - []byte: A byte slice containing the formatted csv data, which is always returned.
//   - error: An error if any part of the generation or file writing process fails.
func ExportBlockInfo(indexData []audio.IndexEntry, outputPath string, sampleRate float64) ([]byte, error) {
	if sampleRate <= 0 {
		return nil, fmt.Errorf("invalid sample rate: %f", sampleRate)
	}

	csvBuffer := new(bytes.Buffer)
	w := tabwriter.NewWriter(csvBuffer, 0, 8, 2, ' ', 0)

	// generate human-readable table with | for visual separated with leading and trailing tab
	_, err := fmt.Fprintln(w, "start_time\t|\tend_time\t|\tblock\t|\tidx_tag\t|\thex_start_time\t|\tfile\t")
	if err != nil {
		return nil, fmt.Errorf("error writing csv header: %w", err)
	}

	blockCount := 0
	i := 0
	// loop through index entries, grouping them into logical blocks
	for i < len(indexData) {
		groupInfo := _getGroupedBlockInfo(indexData, i, sampleRate) // use helper

		if groupInfo.IsBlock {
			wavFileName := fmt.Sprintf("block_%03d_%s.wav", blockCount, groupInfo.BlockType)
			hexStart := fmt.Sprintf("0x%08x", groupInfo.StartEntry.StartPosition)
			// sanitize tag for tabs/newlines - better safe than sorry. people do mad stuff sometimes. bwbahbhaha
			safeIDXTag := strings.ReplaceAll(groupInfo.StartEntry.IDXTag, "\t", " ")
			safeIDXTag = strings.ReplaceAll(safeIDXTag, "\n", " ")
			safeIDXTag = strings.ReplaceAll(safeIDXTag, "|", " ")

			// write line to buffer - use \t for columns, | as visual separator and trailing tab + newline
			_, err = fmt.Fprintf(w, "%.6f\t|\t%.6f\t|\t%s\t|\t%s\t|\t%s\t|\t%s\t\n",
				groupInfo.StartEntry.StartTime,
				groupInfo.BlockEndTime,
				groupInfo.BlockType,
				safeIDXTag,
				hexStart,
				wavFileName,
			)
			// error check per row
			if err != nil {
				return nil, fmt.Errorf("error writing csv data row %d: %w", blockCount, err)
			}

			blockCount++
		}
		i += groupInfo.ConsumedEntries
	}

	// flush tabwriter to ensure all data is processed and aligned in the buffer
	err = w.Flush()
	if err != nil {
		return nil, fmt.Errorf("error flushing tabwriter: %w", err)
	}

	// write file
	if outputPath != "" {
		if err := os.WriteFile(outputPath, csvBuffer.Bytes(), 0644); err != nil {
			return nil, fmt.Errorf("error writing csv file %s: %w", outputPath, err)
		}
	}

	return csvBuffer.Bytes(), nil
}
