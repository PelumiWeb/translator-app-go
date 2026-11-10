"""Checks that one privacy manifest declares everything another one does.

Usage: privacy-manifest-covers.py <covering.xcprivacy> <covered.xcprivacy>

fetch-mlkit.sh ships one of Google's three ML Kit privacy manifests for all three; this stops it if a new
version of the others declares something the shipped one does not: an API reason, a kind of data collected
(with the same purposes, linking and tracking), a tracking domain, or tracking itself.
"""
import plistlib
import sys


def declarations(path: str) -> set:
    with open(path, "rb") as file:
        manifest = plistlib.load(file)
    found = set()
    for api in manifest.get("NSPrivacyAccessedAPITypes", []):
        for reason in api.get("NSPrivacyAccessedAPITypeReasons", []):
            found.add(("API", api["NSPrivacyAccessedAPIType"], reason))
    for data in manifest.get("NSPrivacyCollectedDataTypes", []):
        for purpose in data.get("NSPrivacyCollectedDataTypePurposes", []):
            found.add((
                "data",
                data["NSPrivacyCollectedDataType"],
                purpose,
                data.get("NSPrivacyCollectedDataTypeLinked", False),
                data.get("NSPrivacyCollectedDataTypeTracking", False),
            ))
    for domain in manifest.get("NSPrivacyTrackingDomains", []):
        found.add(("tracking domain", domain))
    if manifest.get("NSPrivacyTracking", False):
        found.add(("tracking",))
    return found


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    covering, covered = sys.argv[1], sys.argv[2]
    missing = declarations(covered) - declarations(covering)
    if missing:
        lines = "\n".join(f"  {' '.join(map(str, item))}" for item in sorted(missing, key=str))
        raise SystemExit(f"{covering} does not declare what {covered} does:\n{lines}")


if __name__ == "__main__":
    main()
