#!/bin/bash
#
# Builds the iPhone app for the App Store and uploads it to TestFlight, signed
# with an Apple Distribution certificate and an App Store provisioning profile.
# The GitHub workflow (.github/workflows/testflight.yml) puts the certificate
# in a keychain and runs this through make; on a Mac whose keychain already
# holds the certificate it runs the same:
#
#   PROFILE_PATH=~/Downloads/Shinar_App_Store.mobileprovision \
#   ASC_KEY_PATH=~/keys/AuthKey_ABC123.p8 ASC_KEY_ID=ABC123 ASC_ISSUER_ID=... \
#   BACKEND_URL=http://OgundipePelumi.local:8080 make ios-testflight
#
# Signing is spelled out rather than automatic: on a fresh machine, automatic
# signing tries to make a development certificate it then cannot use. The API
# key is only for the upload. ADR 0022.
set -euo pipefail

: "${PROFILE_PATH:?the App Store provisioning profile (.mobileprovision)}"
: "${ASC_KEY_PATH:?the App Store Connect API key (.p8)}"
: "${ASC_KEY_ID:?the API key ID}"
: "${ASC_ISSUER_ID:?the API key issuer ID}"
: "${BACKEND_URL:?the server address the build talks to}"

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
WORK=$ROOT/ios/build/testflight
TEAM=4USZRKF452
BUNDLE_ID=com.pelumi.shinar

rm -rf "$WORK"
mkdir -p "$WORK"

# Xcode finds a profile by its UUID, in its own folder.
security cms -D -i "$PROFILE_PATH" > "$WORK/profile.plist"
uuid=$(/usr/libexec/PlistBuddy -c 'Print :UUID' "$WORK/profile.plist")
profiles="$HOME/Library/Developer/Xcode/UserData/Provisioning Profiles"
mkdir -p "$profiles"
cp "$PROFILE_PATH" "$profiles/$uuid.mobileprovision"

echo "Archiving for the App Store, talking to $BACKEND_URL"
xcodebuild archive \
    -project "$ROOT/ios/PTranslate.xcodeproj" -scheme PTranslate -configuration Release \
    -destination 'generic/platform=iOS' -archivePath "$WORK/Shinar.xcarchive" -quiet \
    CODE_SIGN_STYLE=Manual DEVELOPMENT_TEAM="$TEAM" \
    CODE_SIGN_IDENTITY="Apple Distribution" PROVISIONING_PROFILE_SPECIFIER="$uuid" \
    BACKEND_URL="$BACKEND_URL"

# "upload" sends the build to App Store Connect instead of writing an .ipa.
# Xcode gives it the next build number App Store Connect expects, so builds
# from here and from Xcode on a Mac never collide.
cat > "$WORK/ExportOptions.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>method</key>
	<string>app-store-connect</string>
	<key>destination</key>
	<string>upload</string>
	<key>teamID</key>
	<string>$TEAM</string>
	<key>signingStyle</key>
	<string>manual</string>
	<key>signingCertificate</key>
	<string>Apple Distribution</string>
	<key>provisioningProfiles</key>
	<dict>
		<key>$BUNDLE_ID</key>
		<string>$uuid</string>
	</dict>
	<key>manageAppVersionAndBuildNumber</key>
	<true/>
	<key>uploadSymbols</key>
	<true/>
</dict>
</plist>
EOF

echo "Uploading to App Store Connect"
xcodebuild -exportArchive \
    -archivePath "$WORK/Shinar.xcarchive" -exportOptionsPlist "$WORK/ExportOptions.plist" \
    -exportPath "$WORK/export" -allowProvisioningUpdates \
    -authenticationKeyPath "$ASC_KEY_PATH" -authenticationKeyID "$ASC_KEY_ID" \
    -authenticationKeyIssuerID "$ASC_ISSUER_ID"
echo "Uploaded. It reaches TestFlight once App Store Connect has processed it, usually within half an hour."
