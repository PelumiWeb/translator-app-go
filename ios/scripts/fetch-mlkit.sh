#!/bin/bash
#
# Fetches Google's ML Kit translation libraries into ios/MLKit/Frameworks, as the
# xcframeworks the local MLKit package (ios/MLKit/Package.swift) links, and the
# resource bundle the app copies.
#
# They are the files Google publishes for CocoaPods, downloaded from Google's own
# server and checked against the checksums below; CocoaPods itself is not used.
# Each xcframework holds Google's arm64 iPhone code as Google built it, and a copy
# of the same code marked for the simulator (retag-for-simulator.py). The x86_64
# code, for Intel simulators, is left out.
#
# Google ships them as static frameworks. They are repackaged as static libraries
# with their headers, because Xcode embeds a package's frameworks in the app even
# when they are static: it would add three empty frameworks to the app, each
# needing an Info.plist and missing debug symbols when the build is uploaded.
#
# It also builds the three small parts of GoogleToolboxForMac that ML Kit calls,
# from Google's source, at the version ML Kit asks for.
#
# Does nothing when the frameworks already match this script. ADR 0024 has the
# reasons.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
SCRIPTS=$ROOT/ios/scripts
OUT=$ROOT/ios/MLKit/Frameworks
WORK=$ROOT/ios/build/mlkit
STAMP=$OUT/stamp

# The app's deployment target.
MIN_IOS=18.0

# Name, version, the folder Google keeps that version in, and the SHA-256 of its
# archive. The folders come from each version's podspec (github.com/CocoaPods/Specs);
# the checksums were taken when the version was adopted. MLKitTranslate needs the
# other two, at these versions.
LIBRARIES=(
    "MLKitTranslate 8.0.0 2beeb631ff9efd40 825bcf450309febeeeb8cfd886269115ce67bc46751116f70d3f81c5d3817bf3"
    "MLKitNaturalLanguage 10.0.0 16c4bb76b6337f56 51ecfefdbe2ee3e479809c8945d602bff5581b38ab63239980794688f4e01309"
    "MLKitCommon 14.0.0 00f258dabdb58dfa 5c35abbd10a9412c4d1765e67999dd16bc99592ebaa4f993aea48a1c543c7fe2"
)

# GoogleToolboxForMac, at the tag ML Kit's podspecs ask for and the commit that
# tag pointed to when it was adopted. Its Swift package (from 6.0) cannot be used:
# one part holds only headers, which Xcode cannot link
# (github.com/swiftlang/swift-package-manager/issues/5706).
GTM_TAG=v4.2.1
GTM_COMMIT=d1b4c31d71ad9ea2573cf42a2cfa027a34a6eac0

# Changing this script or the ones it runs rebuilds everything.
wanted=$(shasum -a 256 "$0" "$SCRIPTS/retag-for-simulator.py" "$SCRIPTS/privacy-manifest-covers.py" | awk '{print $1}')
if [[ -f $STAMP && $(cat "$STAMP") == "$wanted" ]]; then
    echo "ML Kit is up to date (MLKitTranslate $(echo "${LIBRARIES[0]}" | awk '{print $2}'))"
    exit 0
fi

rm -rf "$WORK" "$OUT"
mkdir -p "$WORK" "$OUT"

for library in "${LIBRARIES[@]}"; do
    read -r name version folder sha256 <<<"$library"
    archive=$WORK/$name-$version.tar.gz
    echo "Fetching $name $version"
    curl --fail --silent --show-error --location --retry 3 -o "$archive" \
        "https://dl.google.com/dl/cpdc/$folder/$name-$version.tar.gz"
    echo "$sha256  $archive" | shasum -a 256 --check --quiet

    mkdir -p "$WORK/$name"
    tar -xzf "$archive" -C "$WORK/$name"
    framework=$WORK/$name/Frameworks/$name.framework

    # The headers, in a folder named after the module, as Google's own headers
    # include each other (<MLKitCommon/MLKModelManager.h>), with the framework's
    # module map made into a library's.
    headers=$WORK/$name/headers/$name
    mkdir -p "$headers"
    if [[ -d $framework/Headers ]]; then cp "$framework"/Headers/*.h "$headers/"; fi
    sed 's/^framework module /module /' "$framework/Modules/module.modulemap" > "$headers/module.modulemap"

    # Google's arm64 code as it is, and a copy marked for the simulator. Each is a
    # static library: MLKitTranslate's code is a single object file, which libtool
    # wraps in one; the others already are.
    lipo "$framework/$name" -thin arm64 -output "$WORK/$name/iphoneos.bin"
    python3 -I "$SCRIPTS/retag-for-simulator.py" "$WORK/$name/iphoneos.bin" "$WORK/$name/iphonesimulator.bin"
    for sdk in iphoneos iphonesimulator; do
        library=$WORK/$name/$sdk/lib$name.a
        mkdir -p "$(dirname "$library")"
        if [[ $(head -c 8 "$WORK/$name/$sdk.bin") == '!<arch>' ]]; then
            cp "$WORK/$name/$sdk.bin" "$library"
        else
            libtool -static -o "$library" "$WORK/$name/$sdk.bin"
        fi
    done

    xcodebuild -create-xcframework \
        -library "$WORK/$name/iphoneos/lib$name.a" -headers "$WORK/$name/headers" \
        -library "$WORK/$name/iphonesimulator/lib$name.a" -headers "$WORK/$name/headers" \
        -output "$OUT/$name.xcframework" >/dev/null
done

# The translator reads two small settings files (its list of language packs) from
# this bundle, which it looks for in the app's own bundle.
bundle=$OUT/MLKitTranslate_resource.bundle
cp -R "$WORK/MLKitTranslate/Frameworks/MLKitTranslate.framework/MLKitTranslate_resource.bundle" "$bundle"
# Google's privacy manifest goes in the same bundle, since the libraries carry
# none into the app. MLKitTranslate's declares everything the other two do, and
# more; the script stops if that ever changes.
for name in MLKitNaturalLanguage MLKitCommon; do
    python3 -I "$SCRIPTS/privacy-manifest-covers.py" \
        "$WORK/MLKitTranslate/Frameworks/MLKitTranslate.framework/PrivacyInfo.xcprivacy" \
        "$WORK/$name/Frameworks/$name.framework/PrivacyInfo.xcprivacy"
done
cp "$WORK/MLKitTranslate/Frameworks/MLKitTranslate.framework/PrivacyInfo.xcprivacy" "$bundle/"

echo "Building GoogleToolboxForMac ${GTM_TAG#v}"
git -c advice.detachedHead=false clone --quiet --depth 1 --branch "$GTM_TAG" \
    https://github.com/google/google-toolbox-for-mac.git "$WORK/gtm"
if [[ $(git -C "$WORK/gtm" rev-parse HEAD) != "$GTM_COMMIT" ]]; then
    echo "GoogleToolboxForMac $GTM_TAG is not the commit this script expects" >&2
    exit 1
fi
for sdk in iphoneos iphonesimulator; do
    target=arm64-apple-ios$MIN_IOS
    [[ $sdk == iphonesimulator ]] && target+=-simulator
    mkdir -p "$WORK/gtm-$sdk"
    # Logger and StringEncoding manage memory by hand in this version; only
    # NSData+zlib is written for ARC. The podspec builds them the same way.
    for part in GTMLogger:-fno-objc-arc GTMStringEncoding:-fno-objc-arc GTMNSData+zlib:-fobjc-arc; do
        file=${part%%:*}
        xcrun --sdk "$sdk" clang -target "$target" -Os -g "${part#*:}" \
            -I "$WORK/gtm" -c "$WORK/gtm/Foundation/$file.m" -o "$WORK/gtm-$sdk/$file.o"
    done
    libtool -static -o "$WORK/gtm-$sdk/libGoogleToolboxForMac.a" "$WORK/gtm-$sdk"/*.o
done
# No headers: nothing in the app calls it, only Google's compiled code does.
xcodebuild -create-xcframework \
    -library "$WORK/gtm-iphoneos/libGoogleToolboxForMac.a" \
    -library "$WORK/gtm-iphonesimulator/libGoogleToolboxForMac.a" \
    -output "$OUT/GoogleToolboxForMac.xcframework" >/dev/null

echo "$wanted" > "$STAMP"
# The downloads and copies take a few hundred megabytes, and the dev machine is
# short on disk.
rm -rf "$WORK"
echo "Fetched ML Kit into $OUT"
