package loaders

// Fmt represents the format parameters for a specific tape loader.
type Fmt struct {
	Name     string // format name
	Endian   int    // byte endianness, 0=LSbF, 1=MSbF
	TP       int    // threshold pulsewidth (if applicable)
	SP       int    // ideal short pulsewidth
	MP       int    // ideal medium pulsewidth (if applicable)
	LP       int    // ideal long pulsewidth
	PV       int    // pilot value
	SV       int    // sync value
	PMin     int    // minimum pilots that should be present.
	PMax     int    // maximum pilots that should be present.
	HasCS    int    // flag, provides checksums, 1=yes, 0=no.
}

const (
	NA = -1 // Not Applicable
	VV = -1 // A variable value is used.
	XX = -1 // Don't care.

	LSbF = 0 // Least Significant bit First.
	MSbF = 1 // Most Significant bit First.

	CSYES = 1 // A checksum is used.
	CSNO  = 0 // A checksum is not used.

	// Loader Type IDs (from tapclean's mydefs.h enum)
	LT_NONE  = 0
	GAP      = 1
	PAUSE    = 2
	CBM_HEAD = 3
	CBM_DATA = 4
	TT_HEAD  = 5
	TT_DATA  = 6
)

// ft array contains info about various tape formats, mirroring tapclean's ft[] array.
// Only relevant entries for CBM and Turbotape are included for now.
var Ft = []Fmt{
	// Index 0-2 (LT_NONE, GAP, PAUSE) are placeholders.
	{},
	{},
	{},

	// CBM_HEAD (C64 ROM-TAPE HEADER) - Index 3
	{Name: "C64 ROM-TAPE HEADER", Endian: LSbF, TP: NA, SP: 0x30, MP: 0x42, LP: 0x56, PV: NA, SV: NA, PMin: 50, PMax: NA, HasCS: CSYES},
	// CBM_DATA (C64 ROM-TAPE DATA) - Index 4
	{Name: "C64 ROM-TAPE DATA", Endian: LSbF, TP: NA, SP: 0x30, MP: 0x42, LP: 0x56, PV: NA, SV: NA, PMin: 50, PMax: NA, HasCS: CSYES},
	// TT_HEAD (TURBOTAPE-250 HEADER) - Index 5
	{Name: "TURBOTAPE-250 HEADER", Endian: MSbF, TP: 0x20, SP: 0x1A, MP: NA, LP: 0x28, PV: 0x02, SV: 0x09, PMin: 50, PMax: NA, HasCS: CSNO},
	// TT_DATA (TURBOTAPE-250 DATA) - Index 6
	{Name: "TURBOTAPE-250 DATA", Endian: MSbF, TP: 0x20, SP: 0x1A, MP: NA, LP: 0x28, PV: 0x02, SV: 0x09, PMin: 50, PMax: NA, HasCS: CSYES},
}