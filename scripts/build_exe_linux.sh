#!/bin/bash
# builds the linux executable for go_chirp_the_tap
# run from the project root directory: ./scripts/build_exe_linux.sh 
# assumes go is installed.

# configuration
output_dir="./build/linux_amd64"
output_name="go_chirp_the_tap"
output_path="$output_dir/$output_name"
source_path="./cmd/main.go" # main package source

# start build
echo "Building go_chirp_the_tap for Linux ($output_name)..."

# create output directory
mkdir -p "$output_dir"
if [ $? -ne 0 ]; then
    echo "error: failed to create output directory '$output_dir'."
    exit 1
fi

# run go build
go build -v -ldflags="-s -w" -o "$output_path" "$source_path"

# check build result
if [ $? -eq 0 ]; then
    echo "Build successful: $output_path"
    echo "Usage: $output_path [-clock pal|ntsc] [-format wav|pcm] [-cpk] [-csv] input.tap" 
else
    echo "Build failed!"
    exit 1 # exit with error code on failure
fi

# set executable permissions
chmod +x "$output_path"
if [ $? -ne 0 ]; then
    echo "warning: failed to set executable permissions on '$output_path'."
fi
