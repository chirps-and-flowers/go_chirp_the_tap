// internal/audio/wava.go
package audio

import (
	"encoding/binary"
	"io"
	"os"
)

const (
	// standard wav header constants
	riffChunkID   = "RIFF"
	waveFormatID  = "WAVE"
	fmtChunkID    = "fmt "
	dataChunkID   = "data"
	pcmFormatTag  = 1  // pcm audio format
	numChannels   = 1  // mono audio
	bitsPerSample = 8  // 8-bit audio
	blockAlign    = 1  // numChannels * bitsPerSample/8
	fmtChunkSize  = 16 // size of the fmt chunk
)

// WriteWAVHeader writes a wav header to the given writer
func WriteWAVHeader(w io.Writer, sampleRate int, dataSize int) error {
	// Calculate sizes
	fileSize := 36 + dataSize // total file size minus 8 bytes for the riff header

	// write riff header
	if err := writeString(w, riffChunkID); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(fileSize)); err != nil {
		return err
	}
	if err := writeString(w, waveFormatID); err != nil {
		return err
	}

	// write format chunk
	if err := writeString(w, fmtChunkID); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(fmtChunkSize)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(pcmFormatTag)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(numChannels)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(sampleRate)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(sampleRate*numChannels*bitsPerSample/8)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(blockAlign)); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint16(bitsPerSample)); err != nil {
		return err
	}

	// write data chunk header
	if err := writeString(w, dataChunkID); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(dataSize)); err != nil {
		return err
	}

	return nil
}

// WriteWAVFile creates a wav file from pcm data
func WriteWAVFile(filename string, pcmData []byte, sampleRate int) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := WriteWAVHeader(file, sampleRate, len(pcmData)); err != nil {
		return err
	}

	_, err = file.Write(pcmData)
	return err
}

func writeString(w io.Writer, s string) error {
	_, err := w.Write([]byte(s))
	return err
}
