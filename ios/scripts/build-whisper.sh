#!/bin/bash
#
# Builds the pinned whisper.cpp submodule for iOS into ios/build/whisper.xcframework:
# one static library for iPhones and one for the simulator (both arm64), with
# the headers and a module map so Swift can `import whisper`.
#
# Does nothing when the xcframework already matches the submodule's commit.
# ADR 0004 (addendum) has the reasons for the settings.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
WHISPER=$ROOT/third_party/whisper.cpp
OUT=$ROOT/ios/build
XCFRAMEWORK=$OUT/whisper.xcframework
STAMP=$OUT/whisper.commit

# The app's deployment target. iOS 26 runs on the A13 and later, whose CPU has
# the dot-product and half-precision instructions ggml's fast paths use. Left
# to itself the compiler would assume the oldest 64-bit iPhone and skip them.
MIN_IOS=26.0
ARM_ARCH=armv8.4-a+fp16+dotprod

commit=$(git -C "$WHISPER" rev-parse HEAD)
if [[ -d $XCFRAMEWORK && -f $STAMP && $(cat "$STAMP") == "$commit" ]]; then
    echo "whisper.xcframework is up to date (whisper.cpp $(git -C "$WHISPER" describe --tags))"
    exit 0
fi

# Builds whisper.cpp for one SDK (iphoneos or iphonesimulator) and merges its
# static libraries into one, since an xcframework takes one library per
# platform.
build() {
    local sdk=$1
    local dir=$OUT/whisper-$sdk
    cmake -S "$WHISPER" -B "$dir" -G Ninja \
        -DCMAKE_SYSTEM_NAME=iOS \
        -DCMAKE_OSX_SYSROOT="$sdk" \
        -DCMAKE_OSX_ARCHITECTURES=arm64 \
        -DCMAKE_OSX_DEPLOYMENT_TARGET=$MIN_IOS \
        -DCMAKE_BUILD_TYPE=Release \
        -DBUILD_SHARED_LIBS=OFF \
        -DWHISPER_BUILD_TESTS=OFF \
        -DWHISPER_BUILD_EXAMPLES=OFF \
        -DWHISPER_BUILD_SERVER=OFF \
        -DGGML_METAL=OFF \
        -DGGML_NATIVE=OFF \
        -DGGML_CPU_ARM_ARCH=$ARM_ARCH \
        -DGGML_OPENMP=OFF \
        -DGGML_CCACHE=OFF
    # Four jobs, not one per core: the dev machine has 8 GB of memory.
    cmake --build "$dir" -j 4
    find "$dir" -name '*.a' ! -name 'libwhisper-ios.a' -print0 \
        | xargs -0 libtool -static -no_warning_for_no_symbols -o "$dir/libwhisper-ios.a"
}

build iphoneos
build iphonesimulator

# The headers Swift sees: whisper.h and the ggml headers it includes. They sit
# in a folder named after the module, which is where Clang looks for a module
# map, and which keeps them apart from other libraries' headers.
HEADERS=$OUT/whisper-headers
rm -rf "$HEADERS"
mkdir -p "$HEADERS/whisper"
cp "$WHISPER/include/whisper.h" "$HEADERS/whisper/"
for header in ggml.h ggml-cpu.h ggml-backend.h ggml-alloc.h; do
    cp "$WHISPER/ggml/include/$header" "$HEADERS/whisper/"
done
cat > "$HEADERS/whisper/module.modulemap" <<'EOF'
module whisper {
    header "whisper.h"
    export *
    link "c++"
    link framework "Accelerate"
}
EOF

rm -rf "$XCFRAMEWORK"
xcodebuild -create-xcframework \
    -library "$OUT/whisper-iphoneos/libwhisper-ios.a" -headers "$HEADERS" \
    -library "$OUT/whisper-iphonesimulator/libwhisper-ios.a" -headers "$HEADERS" \
    -output "$XCFRAMEWORK"
echo "$commit" > "$STAMP"

# The build folders hold a few hundred megabytes of object files, and the dev
# machine is short on disk. They are only needed again when the submodule moves.
rm -rf "$OUT/whisper-iphoneos" "$OUT/whisper-iphonesimulator" "$HEADERS"
echo "Built $XCFRAMEWORK"
