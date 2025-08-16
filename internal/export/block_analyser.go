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

	// Helper to check if a block type is a lead type
	isLeadType := func(blockType string) bool {
		return blockType == "lead" || blockType == "tt_head" || blockType == "cbm_head"
	}

	// Helper to check if a block type is a data type
	isDataType := func(blockType string) bool {
		return blockType == "data" || blockType == "tt_data" || blockType == "cbm_data"
	}

	// block grouping logic: check specific patterns of adjacent entry types.
	// pauses alone are not considered exportable blocks in this logic.

	// Case 1: Current is a lead type (lead, tt_head, cbm_lead)
	if isLeadType(current.Type) {
		// If followed by a pause, group lead + pause
		if hasNext && next.Type == "pause" {
			info.IsBlock = true
			info.BlockType = current.Type // Preserve specific lead type (tt_head, cbm_lead)
			info.StartEntry = current
			info.EndEntry = next
			info.BlockEndTime = _calculateEndTime(next, sampleRate)
			info.ConsumedEntries = 2
		} else {
			// If not followed by a pause, treat the lead as a standalone block
			// This covers tt_head followed by tt_data, or cbm_lead followed by cbm_data, etc.
			info.IsBlock = true
			info.BlockType = current.Type // Preserve specific lead type
			info.StartEntry = current
			info.EndEntry = current
			info.BlockEndTime = _calculateEndTime(current, sampleRate)
			info.ConsumedEntries = 1
		}
	} else if isDataType(current.Type) {
		// Case 2: Current is a data type (data, tt_data, cbm_data)
		// If followed by a pause, group data + pause
		if hasNext && next.Type == "pause" {
			info.IsBlock = true
			info.BlockType = current.Type // Preserve specific data type (tt_data, cbm_data)
			info.StartEntry = current
			info.EndEntry = next
			info.BlockEndTime = _calculateEndTime(next, sampleRate)
			info.ConsumedEntries = 2
		} else if hasNext && isLeadType(next.Type) {
			// If followed by a lead, treat data as a standalone block (lead will be processed next)
			info.IsBlock = true
			info.BlockType = current.Type // Preserve specific data type
			info.StartEntry = current
			info.EndEntry = current
			info.BlockEndTime = _calculateEndTime(current, sampleRate) // End at current block's end
			info.ConsumedEntries = 1
		} else if hasNext && next.Type == "tt_trailer" { // NEW: tt_data followed by tt_trailer
			info.IsBlock = true
			info.BlockType = current.Type // Keep as tt_data, but extend its range
			info.StartEntry = current
			info.EndEntry = next // End at the end of the trailer
			info.BlockEndTime = _calculateEndTime(next, sampleRate)
			info.ConsumedEntries = 2 // Consume both tt_data and tt_trailer
		} else {
			// If it's the last entry or followed by something else, treat data as a standalone block
			info.IsBlock = true
			info.BlockType = current.Type // Preserve specific data type
			info.StartEntry = current
			info.EndEntry = current
			info.BlockEndTime = _calculateEndTime(current, sampleRate)
			info.ConsumedEntries = 1
		}
	} else if current.Type == "tt_trailer" {
		// Case 4: Current is a Turbotape trailer.
		// Treat it as a standalone exportable block.
		info.IsBlock = true
		info.BlockType = current.Type // Preserve "tt_trailer"
		info.StartEntry = current
		info.EndEntry = current
		info.BlockEndTime = _calculateEndTime(current, sampleRate)
		info.ConsumedEntries = 1
	} else if current.Type == "pause" {
		// Case 3: Current is a pause. Pauses are generally not exported as standalone blocks
		// unless they are part of a lead+pause or data+pause group.
		// If a pause is encountered here, it means it wasn't grouped with a preceding lead/data.
		// We just consume it and don't mark it as an exportable block.
		info.IsBlock = false
		info.ConsumedEntries = 1
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
