package constants

const (
	// clock frequencies
	ClockPAL  = 985248.0
	ClockNTSC = 1022727.0

	// clock standard names
	ClockStandardPAL     = "PAL"
	ClockStandardNTSC    = "NTSC"
	ClockStandardUnknown = "Unknown"

	// audio generator constants
	MinLeadToneLength   = 25000 // minimum consecutive bytes to consider as a lead tone
	RequiredConsistency = 0.9   // at least 90% of bytes must be the same value. we allow leniency here due to poor quality .tap files
	MaxOffset           = 1500  // .idx files are read and joined with the index generated here. sometimes it does not match, hence the need for a lenient approach (allow an offset in tap file bytes)

	// .tap file constants
	TapHeaderSize        = 20 // header size for C64-TAPE-RAW v0/v1
	TapSignatureC64      = "C64-TAPE-RAW"
	TapMaxVersionSupport = 1 // only support for tap version 0 and 1

	// sample rate
	SampleRate = 44100.0

	// pulse constants
	PULSES_IN_BYTE = 8 // Default pulses per byte for generic readByte
	PULSES_IN_CBM_BYTE = 20 // Pulses per byte for CBM format
	DefaultTolerance = 16 // Default bit reading tolerance, from tapclean's DEFTOL
)
