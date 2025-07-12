#!/bin/bash
# builds the android aar library for go_chirp_the_tap
# run from the project root directory: ./scripts/build_aar_android.sh  
# assumes gomobile is installed and NDK env var is set.

# configuration
output_dir="./build/android"
output_name="go_chirp_the_tap.aar"
output_path="$output_dir/$output_name"
package_path="./mobile" # go package containing exported api

# android ndk check (essential for gomobile)
if [ -z "$ANDROID_NDK_HOME" ]; then
    echo "error: ANDROID_NDK_HOME environment variable is not set."
    exit 1
fi

# start build
echo "Building go_chirp_the_tap for Android ($output_name)..."

# create output directory
mkdir -p "$output_dir"
if [ $? -ne 0 ]; then
    echo "error: failed to create output directory '$output_dir'."
    exit 1
fi

# run gomobile bind
gomobile bind -v\
    -target=android/arm,android/arm64 \
    -androidapi 21 \
    -o "$output_path" \
    "$package_path"

# check build result
if [ $? -eq 0 ]; then
    echo "Build successful: $output_path"
else
    echo "Build failed!"
    exit 1 # exit with error code on failure
fi