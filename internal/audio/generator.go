package audio

import (
	"fmt"
	"go_chirp_the_tap/internal/constants"
	"go_chirp_the_tap/internal/idx"
	"go_chirp_the_tap/internal/loaders" // Import the loaders package
	"math"
	"sort"
)

// struct holding metadata for each data segment, ie. a block detected during .tap processing.
type IndexEntry struct {
	StartSample   int     // starting sample index within generated pcm data
	EndSample     int     // ending sample index within generated pcm data (inclusive)
	Type          string  // holds block types: "lead", "data", "pause"
	StartTime     float64 // time for block start in seconds
	StartPosition int     // start position in original tap file bytes (includes header offset)
	EndPosition   int     // end position in original tap file bytes (includes header offset)
	IDXTag        string  // holds matching tag from .idx file (set during merge); empty if no file or no match
	RawData       []byte  // raw data of the block
	BlockID       int     // block identifier
	PilotEndPos   int     // end position of the pilot within the raw data
}

// ProcessTAPData converts raw .tap data into PCM samples
// and builds a slice of IndexEntry structs (one per detected block)
// and merges optional IDX data into it
// and returns the resulting slice.
func ProcessTAPData(tapData []byte, version byte, clock, sampleRate float64, idxEntries []idx.IDXEntry) ([]byte, []IndexEntry, error) {
	_ = loaders.Ft[0] // Dummy call to satisfy unused import check during incremental development

	if len(tapData) < constants.TapHeaderSize {
		return nil, nil, fmt.Errorf("tap data too short: %d bytes, expected at least %d", len(tapData), constants.TapHeaderSize)
	}

	// pcmSamples slice starts empty; capacity grows dynamically via append. no pre-allocation
	// was used due to difficulty finding a reliable heuristic for tap files, esp. due to pauses.
	pcmSamples := make([]byte, 0)

	// indexData slice capacity pre-allocated to a fixed size (512) based on observed max entries.
	// this value is generous compared to what test observations showed and avoids reallocations
	// if the number of blocks stays below this limit.
	indexData := make([]IndexEntry, 0, 512) // len=0, cap=512 (fixed)

	currentSample := 0
	currentPosition := 0     // start position from the beginning of tapData
	i := 0                   // current index in tapData
	expectedDataSize := 0    // state for turbotape data blocks
	expectedTrailerSize := 0 // state for turbotape trailer blocks

	// main loop: process tapdata byte stream block by block.
	// 'i' advances based on the number of bytes consumed by each block.
	for i < len(tapData) {
		// mark start position/sample for the current block
		sectionStartSample := currentSample
		sectionStartPosition := currentPosition

		// variables to hold results for the current block
		var blockPCM []byte
		var blockBytesRead int
		var blockType string
		var err error
		var totalCycles uint32
		var blockID int
		var pilotEndPos int // New variable

		if expectedDataSize > 0 {
			// We have a pending data block to process (from a previous tt_head)
			blockType = "tt_data"
			blockBytesRead = expectedDataSize * constants.PULSES_IN_BYTE
			fmt.Printf("DEBUG: ProcessTAPData: Processing expected data block of size %d bytes at offset %d\n", blockBytesRead, i)

			// Generate PCM for the data block
			for k := 0; k < blockBytesRead; k++ {
				if i+k >= len(tapData) {
					fmt.Printf("warning: tap data ended unexpectedly while reading expected data block at offset %d\n", i)
					break
				}
				b := tapData[i+k]
				pulseCycles := uint32(b) * 8
				waveSamples := cyclesToSamples(pulseCycles, clock, sampleRate)
				waveData := generateWave(waveSamples, 127)
				blockPCM = append(blockPCM, waveData...)
				totalCycles += pulseCycles
			}
			expectedDataSize = 0       // Reset the state
			expectedTrailerSize = 7648 // Set for next iteration (Turbotape trailer)

		} else if expectedTrailerSize > 0 {
			// We have a pending Turbotape trailer to process
			blockType = "tt_trailer"
			blockBytesRead = expectedTrailerSize // Trailer size is in pulses
			fmt.Printf("DEBUG: ProcessTAPData: Processing expected Turbotape trailer of size %d bytes at offset %d\n", blockBytesRead, i)

			// Generate PCM for the trailer block
			for k := 0; k < blockBytesRead; k++ {
				if i+k >= len(tapData) {
					fmt.Printf("warning: tap data ended unexpectedly while reading expected Turbotape trailer at offset %d\n", i)
					break
				}
				b := tapData[i+k]
				pulseCycles := uint32(b) * 8
				waveSamples := cyclesToSamples(pulseCycles, clock, sampleRate)
				waveData := generateWave(waveSamples, 127)
				blockPCM = append(blockPCM, waveData...)
				totalCycles += pulseCycles
			}
			expectedTrailerSize = 0 // Reset the state
		} else {
			b := tapData[i]

			// dispatch block processing based on current byte (0 = pause, non-zero = data/lead)
			if b == 0 {
				blockPCM, blockBytesRead, totalCycles, err = _processPauseBlock(tapData, i, version, clock, sampleRate)
				blockType = "pause"
			} else {

				var leadPcm []byte
				var leadBytesRead int
				var leadTotalCycles uint32
				var leadBlockType string
				var leadPsize int

				// Modified call to _processDataLeadBlock
				leadPcm, leadBytesRead, leadTotalCycles, leadBlockType, leadPsize, blockID, pilotEndPos, err = _processDataLeadBlock(tapData, i, clock, sampleRate)

				blockPCM = leadPcm
				blockBytesRead = leadBytesRead
				blockType = leadBlockType
				totalCycles = leadTotalCycles

				if leadBlockType == "tt_head" {
					expectedDataSize = leadPsize // Set for next iteration

				}
			}
		}

		// handle processing errors reported by helper functions
		if err != nil {
			return nil, nil, fmt.Errorf("error processing tap block starting at file offset %d: %w", sectionStartPosition, err)
		}
		// safety check to prevent infinite loop if a block processor returns zero bytes read - probably redundant; better safe than sorry.
		if blockBytesRead <= 0 {
			fmt.Printf("warning: block processing at offset %d returned %d bytes read, stopping.\n", sectionStartPosition, blockBytesRead)
			break
		}

		// append generated audio samples
		if len(blockPCM) > 0 {
			pcmSamples = append(pcmSamples, blockPCM...)
		}

		// update overall progress counters
		currentSample += len(blockPCM)
		currentPosition += blockBytesRead

		// create index entry for the processed block
		indexData = append(indexData, IndexEntry{
			StartSample:   sectionStartSample,
			EndSample:     currentSample - 1,
			Type:          blockType,
			StartTime:     float64(sectionStartSample) / sampleRate,
			StartPosition: sectionStartPosition,
			EndPosition:   currentPosition - 1,
			IDXTag:        "", // tag populated later by merge
			RawData:       tapData[sectionStartPosition:currentPosition],
			BlockID:       blockID,
			PilotEndPos:   pilotEndPos, // New field
		})

		// advance loop counter to the start of the next block
		i += blockBytesRead

	} // end main processing loop

	// merge external idx data before returning
	mergedIndexData := mergeIDXData(indexData, idxEntries)
	return pcmSamples, mergedIndexData, nil
}

// mergeIDXData assigns tags from an external .idx file (idxEntries) to detected blocks (indexData).
// for each idxEntry, it finds the most appropriate block in indexData by comparing the idxEntry's
// byte Position to the block's StartPosition (relative to the original .tap file). a match is
// considered appropriate if the positions are within maxOffset bytes of each other.
func mergeIDXData(indexData []IndexEntry, idxEntries []idx.IDXEntry) []IndexEntry {
	// skip if nothing to merge (no .idx file with entries)
	if len(idxEntries) == 0 || len(indexData) == 0 {
		return indexData
	}

	// sort both slices by position for efficient matching
	sort.Slice(indexData, func(i, j int) bool { return indexData[i].StartPosition < indexData[j].StartPosition })
	sort.Slice(idxEntries, func(i, j int) bool { return idxEntries[i].Position < idxEntries[j].Position })

	k := 0 // index for indexData slice

	// iterate through external .idx entries
	for j := 0; j < len(idxEntries); j++ {
		idxEntry := idxEntries[j]
		targetPos := idxEntry.Position // position from .idx file

		// define the search window around the target position using maxOffset
		minPos := targetPos - constants.MaxOffset
		maxPos := targetPos + constants.MaxOffset

		// advance indexData pointer (k) past entries that are definitely too early
		for k < len(indexData) && indexData[k].EndPosition < minPos {
			k++
		}

		bestMatchIdx := -1                     // index in indexData of the best match found
		minDistance := constants.MaxOffset + 1 // track closest distance found so far

		// search for the best match within the window [minPos, maxPos]
		// iterate starting from k (we don't need to re-check earlier entries)
		for currentK := k; currentK < len(indexData); currentK++ {
			indexEntryToTest := &indexData[currentK]

			// if this detected block starts after our search window, no further matches are possible for this idxEntry
			if indexEntryToTest.StartPosition > maxPos {
				break
			}

			// check if the block overlaps the window and is a relevant type ("data" or "lead" or "tt_lead" or "cbm_head")
			if (indexEntryToTest.Type == "data" || indexEntryToTest.Type == "lead" || indexEntryToTest.Type == "tt_head" || indexEntryToTest.Type == "cbm_head") &&
				indexEntryToTest.StartPosition <= maxPos && // block starts within or before window end
				indexEntryToTest.EndPosition >= minPos { // block ends within or after window start (allows overlap)

				// calculate distance from idx position to detected block start position
				distance := abs(targetPos - indexEntryToTest.StartPosition)

				// if within tolerance and closer than previous best match, update best match
				if distance <= constants.MaxOffset && distance < minDistance {
					minDistance = distance
					bestMatchIdx = currentK
				}
			}
		} // end inner search loop (for currentK)

		// if a suitable match was found, assign the tag
		if bestMatchIdx != -1 {
			indexData[bestMatchIdx].IDXTag = idxEntry.Name
		}
	} // end outer loop (for j)

	// final sort of indexData to ensure canonical order before returning.
	// most likely not needed - but cheap, so why not.
	sort.Slice(indexData, func(i, j int) bool {
		if indexData[i].StartPosition != indexData[j].StartPosition {
			return indexData[i].StartPosition < indexData[j].StartPosition
		}
		// 2nd sort by end position if start is the same
		if indexData[i].EndPosition != indexData[j].EndPosition {
			return indexData[i].EndPosition < indexData[j].EndPosition
		}
		// 3rd tertiary sort by start time if positions are the same
		return indexData[i].StartTime < indexData[j].StartTime
	})

	return indexData
}

// _processPauseBlock handles a tap pause block (identified by starting byte value 0).
// determines duration based on tap version and following bytes and aims to correctly
// process and interpret how both v0 and v1 .tap formats represent pauses (silence),
// while also handling incomplete or truncated files gracefully where possible.
func _processPauseBlock(tapData []byte, i int, version byte, clock, sampleRate float64) (pcm []byte, bytesRead int, cycles uint32, err error) {
	bytesRead = 1 // start with the '0' byte itself
	pauseDurationOffset := i + bytesRead

	// determine pause duration (in cycles) based on version and EOF check
	if pauseDurationOffset+2 >= len(tapData) { // check if there are enough bytes for duration
		if version == 0 {
			cycles = 20000 // default pause for v0 if duration bytes are missing (spec is unclear here)
			// bytesRead remains 1, effectively consuming only the '0'
		} else {
			// v1 requires 3 bytes for duration, hitting EOF is an error
			err = fmt.Errorf("unexpected EOF reading v1 pause duration at offset %d", i)
			return // return immediately with error
		}
	} else {
		// enough bytes exist for duration
		if version == 0 {
			cycles = 20000 // v0 spec uses fixed pause length, ignore duration byte values.
			// these 3 bytes must still be consumed to advance 'i' correctly in the main loop.
			bytesRead += 3
		} else {
			// v1 reads 3 bytes for duration
			cycles = uint32(tapData[pauseDurationOffset]) | (uint32(tapData[pauseDurationOffset+1]) << 8) | (uint32(tapData[pauseDurationOffset+2]) << 16)
			bytesRead += 3 // consume the 3 duration bytes
		}
	}

	// generate audio samples for the pause
	pauseSamples := cyclesToSamples(cycles, clock, sampleRate)
	pcm = _generatePause(pauseSamples) // use helper to generate silent samples
	return pcm, bytesRead, cycles, nil // return generated pcm, bytes consumed, cycles, and nil error
}

// _processDataLeadBlock handles a sequence of non-zero tap bytes, treating it as pulses.
// it also determines if the sequence likely constitutes a leader tone.
func _processDataLeadBlock(tapData []byte, i int, clock, sampleRate float64) (pcm []byte, bytesRead int, totalCycles uint32, blockType string, psize int, blockID int, pilotEndPos int, err error) {
	startOffset := i // remember starting position for lead tone check and error messages

	// Prioritize Turbotape 250 detection
	isTTLead, ttLeadLength, blockID, psize, pilotEndPos := loaders.IsTurbotape250Lead(tapData, startOffset, constants.DefaultTolerance, constants.PULSES_IN_BYTE)
	if isTTLead {
		// Determine block type and calculate total bytes to read
		blockType = "tt_head"
		bytesRead = ttLeadLength
		// pilotEndPos is already set from IsTurbotape250Lead

		fmt.Printf("DEBUG: _processDataLeadBlock: Processing Turbotape 250 lead from offset %d, type: %s, length: %d\n", startOffset, blockType, bytesRead)

		// Generate PCM for the lead portion only
		for k := 0; k < bytesRead; k++ {
			// Ensure we don't read past the end of tapData
			if startOffset+k >= len(tapData) {
				fmt.Printf("warning: tap data ended unexpectedly while reading turbotape lead at offset %d\n", startOffset)
				break
			}
			b := tapData[startOffset+k]
			pulseCycles := uint32(b) * 8
			waveSamples := cyclesToSamples(pulseCycles, clock, sampleRate)
			// generate the square wave for this pulse
			waveData := generateWave(waveSamples, 127)
			pcm = append(pcm, waveData...)
			totalCycles += pulseCycles
		}
		return pcm, bytesRead, totalCycles, blockType, psize, blockID, pilotEndPos, nil
	}

	// Check for CBM lead
	isCBMLead, cbmLeadLength, blockID, psize, pilotEndPos := loaders.IsCBMLead(tapData, startOffset, constants.DefaultTolerance, constants.PULSES_IN_CBM_BYTE)
	if isCBMLead {
		blockType = "cbm_head"
		bytesRead = cbmLeadLength

		fmt.Printf("DEBUG: _processDataLeadBlock: Processing CBM lead from offset %d, type: %s, length: %d\n", startOffset, blockType, bytesRead)

		// Generate PCM for the lead portion only
		for k := 0; k < bytesRead; k++ {
			// Ensure we don't read past the end of tapData
			if startOffset+k >= len(tapData) {
				fmt.Printf("warning: tap data ended unexpectedly while reading cbm lead at offset %d\n", startOffset)
				break
			}
			b := tapData[startOffset+k]
			pulseCycles := uint32(b) * 8
			waveSamples := cyclesToSamples(pulseCycles, clock, sampleRate)
			// generate the square wave for this pulse
			waveData := generateWave(waveSamples, 127)
			pcm = append(pcm, waveData...)
			totalCycles += pulseCycles
		}
		return pcm, bytesRead, totalCycles, blockType, psize, blockID, pilotEndPos, nil
	} else { // Check for CBM data
		isCBMData, cbmDataLength, blockID, psize, pilotEndPos := loaders.IsCBMData(tapData, startOffset, constants.DefaultTolerance, constants.PULSES_IN_CBM_BYTE)
		if isCBMData {
			blockType = "cbm_data"
			bytesRead = cbmDataLength

			fmt.Printf("DEBUG: _processDataLeadBlock: Processing CBM data from offset %d, type: %s, length: %d\n", startOffset, blockType, bytesRead)

			// Generate PCM for the data portion only
			for k := 0; k < bytesRead; k++ {
				// Ensure we don't read past the end of tapData
				if startOffset+k >= len(tapData) {
					fmt.Printf("warning: tap data ended unexpectedly while reading cbm data at offset %d\n", startOffset)
					break
				}
				b := tapData[startOffset+k]
				pulseCycles := uint32(b) * 8
				waveSamples := cyclesToSamples(pulseCycles, clock, sampleRate)
				// generate the square wave for this pulse
				waveData := generateWave(waveSamples, 127)
				pcm = append(pcm, waveData...)
				totalCycles += pulseCycles
			}
			return pcm, bytesRead, totalCycles, blockType, psize, blockID, pilotEndPos, nil
		} else {
			// Check for generic lead tone
			if isLeadTone(tapData, startOffset) {
				blockType = "lead" // Use "lead" for generic lead tones
			} else {
				blockType = "data" // Default to "data" if not a recognized lead
			}
		}
	}

	// pre-allocate pcm slice (estimate capacity)
	pcm = make([]byte, 0, 1024) // initial capacity, will grow as needed

	// loop through consecutive non-zero bytes
	for i < len(tapData) {
		b := tapData[i]
		if b == 0 {
			break // zero byte marks end of data/lead block, start of pause
		}

		// convert tap byte value to cpu cycles (each unit is 8 cycles)
		pulseCycles := uint32(b) * 8
		// convert cycles to number of audio samples
		waveSamples := cyclesToSamples(pulseCycles, clock, sampleRate)
		// generate the square wave for this pulse
		waveData := generateWave(waveSamples, 127)
		// append generated wave to the block's pcm data
		pcm = append(pcm, waveData...)

		totalCycles += pulseCycles // accumulate total cycles for potential use
		bytesRead++                // increment count of tap bytes consumed
		i++                        // advance index in tapData
	}

	// check if any data bytes were actually read
	if bytesRead == 0 {
		// it should not be possible to be true as we check for b!=0 initially, but
		// we keep the check just to make sure something did not botch up unexpectedly.
		err = fmt.Errorf("no data bytes read in data/lead block starting at offset %d", startOffset)
	}

	return pcm, bytesRead, totalCycles, blockType, 0, 0, 0, err
}

// _generatePause generates samples for pause durations using a specific 255/1 pattern
// (one pulse: half high, half low) for the entire pause length.
// note: this pattern deviates from true silence (value 128).
// rationale: this specific pattern is used intentionally because testing showed that
// the abrupt transitions resulting from starting/stopping true silence (128) can
// cause critical loading failures - example: end of P.O.D - Proof of Destruction.
func _generatePause(len int) []byte {
	samples := make([]byte, len)
	// fill first half with high value (255), second half with low value (1)
	for i := range samples { // use range for idiomatic slice loop
		if i < len/2 {
			samples[i] = 255
		} else {
			samples[i] = 1
		}
	}
	return samples
}

// abs returns the absolute value of the integer x.
// note: needed because standard library math.Abs operates on float64.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// cyclesToSamples converts a duration measured in c64 cpu cycles into the
// corresponding number of audio samples at the given sample rate.
func cyclesToSamples(cycles uint32, clock, sampleRate float64) int {
	// calculation logic:
	// 1. determine duration in seconds: time_sec = cycles / clock_hz
	// 2. determine samples needed: samples = time_sec * sample_rate_hz
	// combined formula: samples = (cycles * sampleRate) / clock

	// perform calculation using float64 for precision.
	numSamplesFloat := float64(cycles) * sampleRate / clock

	// use math.Floor to round down, ensuring generated audio doesn't exceed
	// original duration. convert to int because we need a whole number of samples.
	return int(math.Floor(numSamplesFloat))
}

// generateWave creates a square wave for tape pulses.
// 'len' is number of samples, 'amp' is amplitude (0-127).
func generateWave(len int, amp byte) []byte {
	samples := make([]byte, len)
	offset := byte(128) // dc offset for unsigned 8-bit audio
	halfLen := len / 2

	// create a square wave: high for first half, low for second half
	for i := 0; i < len; i++ {
		var y float64
		if i < halfLen {
			y = float64(amp) + float64(offset) // positive amplitude + offset
		} else {
			y = -float64(amp) + float64(offset) // negative amplitude + offset
		}
		// clamp value to valid 8-bit range [0, 255]
		samples[i] = byte(math.Max(0, math.Min(255, y)))
	}
	return samples
}

// isLeadTone checks if data starting at startPos looks like a c64 lead/header tone
// (beeeeeeeeeeeeeeeeeep). It requires a non-zero starting byte (not a pause) and
// verifies that a sequence of consecutive bytes matching the starting byte's value
// is both long enough (minLeadToneLength) and consistent enough (requiredConsistency)
// within the available data.
func isLeadTone(tapData []byte, startPos int) bool {
	// check if there's enough data left for minLeadToneLength requirement
	if startPos+int(constants.MinLeadToneLength) > len(tapData) {
		return false
	}

	// leader tone cannot be represented by 0 bytes (which indicate pauses)
	candidateValue := tapData[startPos]
	if candidateValue == 0 {
		return false
	}

	sameValueCount := 0
	// determine how many bytes to check - either up to minLeadToneLength or end of data
	checkLength := min(len(tapData)-startPos, int(constants.MinLeadToneLength))

	// count consecutive bytes matching the first byte's value
	for j := 0; j < checkLength; j++ {
		// check if the current byte matches the first byte of the sequence
		if tapData[startPos+j] == candidateValue {
			sameValueCount++
		} else {
			// assumes tone is contiguous identical bytes. stop counting if mismatch found.
			break
		}
	}

	// calculate consistency if any matching bytes were found
	if sameValueCount > 0 {
		// consistency is ratio of same bytes found over the actual length checked.
		denominator := float64(checkLength) // use the actual number of bytes checked
		if denominator == 0 {
			// prevent division by zero if checkLength somehow ended up 0
			return false
		}
		consistency := float64(sameValueCount) / denominator

		// check requires both high consistency and that we examined at least the minimum length.
		return consistency >= constants.RequiredConsistency && checkLength >= int(constants.MinLeadToneLength)
	}

	// return false if no matching bytes were found (e.g., if checkLength was 0)
	return false
}
