# go_chirp_the_tap

`go_chirp_the_tap` is a Go-based tool designed to process Commodore 64 `.tap` files and convert them into the **`.cpk` (Chirp Package)** format. This archive format is the project's primary output, created specifically for use with the "Chirp'n TAP" mobile application.

While its main purpose is `.cpk` creation, the tool also provides a command-line interface and mobile library with additional capabilities for `.tap` file processing.

## Core Feature: The `.cpk` (Chirp Package) Format

The main feature of go_chirp_the_tap is the generation of .cpk (Chirp Package) files (GZipped TAR archives) structured with the application **"Chirp'n TAP"** in mind.

*   **`package_manifest.json`**: A JSON file with conversion metadata, including the clock standard (PAL/NTSC), source file name, and other processing parameters.
*   **`blocks.csv`**: An index of all audio blocks extracted from the `.tap` file. It includes timings, block types (lead, data), and any associated tags from an `.idx` file. The format is designed to be human-readable.
*   **Individual `.wav` Blocks**: Each logical block from the original tape (e.g., a program lead/header, a data segment) is saved as its own separate `.wav` file, named sequentially (e.g., `block_000_lead.wav`, `block_001_data.wav`).

This structure allows a frontend application to parse and manage the tape's contents for interactive playback.

## Other Capabilities

*   **Direct Audio Conversion:** Convert `.tap` files directly into a single `.wav` or `.pcm` audio file.
*   **IDX File Support:** Automatically reads an associated `.idx` file (if present) to include meaningful labels for data blocks within blocks.csv.
*   **Clock Speed Support:** Processes `.tap` files based on PAL or NTSC clock speeds.
*   **Mobile Library:** Exposes a dedicated API for integration into mobile applications, which is how the "Chirp'n TAP" app uses it.

## Installation

To build `go_chirp_the_tap` from source, ensure you have Go 1.23 or later installed.

```bash
git clone https://github.com/your-username/go_chirp_the_tap.git
cd go_chirp_the_tap
go build -o go_chirp_the_tap ./cmd/main.go
```

You can also use the provided build scripts:

*   `./scripts/build_exe_linux.sh`: Builds the executable for Linux.
*   `./scripts/build_aar_android.sh`: Builds the Android AAR library for mobile integration.

## Usage

### Command-Line Tool

The command-line tool `go_chirp_the_tap` can be used as follows:

```bash
./go_chirp_the_tap [flags] <tap_file_path>
```

**Flags:**

*   `-cpk`: **(Primary)** Create a CPK package. This is the main intended use.
*   `-format string`: Output format for direct conversion (e.g., `wav`, `pcm`). Default is `wav`.
*   `-csv`: Generate a standalone CSV file of the block index (only if `-cpk` is not used).
*   `-clock string`: Clock speed standard (`pal` or `ntsc`). Default is `pal`.

**Examples:**

Create a `.cpk` package (recommended):
```bash
./go_chirp_the_tap -cpk mytape.tap
```

Convert a TAP file to WAV (NTSC) and generate a standalone CSV:
```bash
./go_chirp_the_tap -format wav -clock ntsc -csv mytape.tap
```

### Project Status

`go_chirp_the_tap` is currently in **Alpha** stage. This means it's an early, experimental release. Expect bugs, incomplete features, and changes that will break backwards compability. I am currently working on core functionality and finalizing the definition for the `.cpk` (Chirp Package) format.

**Known Limitations:**
*   **Builds:** Linux executable builds (`./scripts/build_exe_linux.sh`) and Android AAR library builds (`./scripts/build_aar_android.sh`) are currently verified running on Arch. Other platforms (e.g., Windows executables) are not yet officially supported or tested.

### Planned Features

*   **Windows Support:** Future plans include support for Windows executables (planned for Beta release).

### Mobile Library

The `mobile/api.go` file exposes functions for integrating `go_chirp_the_tap` functionality into mobile applications. Refer to the `mobile/api.go` file for detailed API usage.

## License

This project is licensed under the [LICENSE](LICENSE) file.