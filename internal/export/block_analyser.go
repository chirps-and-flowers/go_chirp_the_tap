// internal/export/block_analyzer.go
package export

import (
	"go_chirp_the_tap/internal/audio"
)

// _groupedBlockInfo holds results from analyzing a sequence of IndexEntry items
// to identify a single logical block suitable for export (e.g., lead+pause, data+pause).
type _groupedBlockInfo struct {
	IsBlock         bool              // true if a valid exportable block was identified
	BlockType       string            // "lead" or "data" (type of the main part of the block)
	StartEntry      *audio.IndexEntry // pointer to the IndexEntry where the exportable block starts
	EndEntry        *audio.IndexEntry // pointer to the IndexEntry where the exportable block ends (inclusive)
	BlockEndTime    float64           // calculated end time (in seconds) for the identified block
	ConsumedEntries int               // how many entries from indexData were consumed (1 or 2)
}

// _getGroupedBlockInfo analyzes the indexData starting at currentIndex to find
// the next logical, exportable block (like lead+pause or data+pause/lead).
// it determines the block type, its start/end entries, calculated end time,
// and how many indexData entries make up this logical block.
func _getGroupedBlockInfo(indexData []audio.IndexEntry, currentIndex int, sampleRate float64) _groupedBlockInfo {
	// default result: assume no block found, consumes only the current entry by default
	info := _groupedBlockInfo{ConsumedEntries: 1, IsBlock: false}
	if currentIndex >= len(indexData) {
		// reached end of data, definitely no block possible.
		return info
	}

	current := &indexData[currentIndex] // pointer to the current entry being examined
	var next *audio.IndexEntry          // pointer for the next entry, if it exists
	hasNext := currentIndex+1 < len(indexData)
	if hasNext {
		next = &indexData[currentIndex+1]
	}

	// block grouping logic: check specific patterns of adjacent entry types.
	// pauses alone are not considered exportable blocks in this logic.

	// case 1: current entry is "lead".
	// an exportable lead block requires an immediately following "pause".
	if current.Type == "lead" {
		if hasNext && next.Type == "pause" {
			info.IsBlock = true                                     // found a lead+pause block
			info.BlockType = "lead"                                 // type is lead
			info.StartEntry = current                               // starts with the lead
			info.EndEntry = next                                    // ends with the pause
			info.BlockEndTime = _calculateEndTime(next, sampleRate) // end time is end of pause
			info.ConsumedEntries = 2                                // consumed lead and pause
		}

	} else if current.Type == "data" {
		// case 2: current entry is "data". check what follows.
		if hasNext {
			// case 2a: data followed by pause.
			if next.Type == "pause" {
				info.IsBlock = true                                     // found a data+pause block
				info.BlockType = "data"                                 // type is data
				info.StartEntry = current                               // starts with data
				info.EndEntry = next                                    // ends with pause
				info.BlockEndTime = _calculateEndTime(next, sampleRate) // end time is end of pause
				info.ConsumedEntries = 2                                // consumed data and pause
			} else if next.Type == "lead" {
				// case 2b: data followed by lead
				info.IsBlock = true                // found a data block
				info.BlockType = "data"            // type is data
				info.StartEntry = current          // starts with data
				info.EndEntry = current            // ends with data (doesn't include the following lead)
				info.BlockEndTime = next.StartTime // end time is effectively the start of the next block
				info.ConsumedEntries = 1           // only consumed the data entry
			}

		} else {
			// case 3: final data block (current is data and last entry).
			info.IsBlock = true                                        // found a data block
			info.BlockType = "data"                                    // type is data
			info.StartEntry = current                                  // starts with data
			info.EndEntry = current                                    // ends with data
			info.BlockEndTime = _calculateEndTime(current, sampleRate) // end time is end of this block
			info.ConsumedEntries = 1                                   // only consumed this data entry
		}
	}

	// check: ensure calculated block end time is not earlier than the start entry's end time.
	// this could happen in case 2b where BlockEndTime is next.StartTime.
	if info.IsBlock {
		startEntryEndTime := _calculateEndTime(info.StartEntry, sampleRate)
		if info.BlockEndTime < startEntryEndTime {
			// if next block started before this one ended (unlikely), use this block's end time.
			info.BlockEndTime = startEntryEndTime
		}
	}

	return info
}

// _calculateEndTime computes the precise end time of an index entry based on its samples.
func _calculateEndTime(entry *audio.IndexEntry, sampleRate float64) float64 {
	// handle invalid samplerate or entry data gracefully
	if sampleRate <= 0 || entry == nil {
		// cannot calculate, return start time or zero? returning start time implies zero duration.
		if entry != nil {
			return entry.StartTime
		}
		return 0.0
	}
	// handle case where end sample might be less than start sample (shouldn't happen)
	if entry.EndSample < entry.StartSample {
		return entry.StartTime // treat as zero duration
	}

	// calculate duration in samples (+1 because start/end are inclusive indices)
	durationSamples := float64(entry.EndSample - entry.StartSample + 1)
	// calculate duration in seconds and add to start time
	return entry.StartTime + (durationSamples / sampleRate)
}
