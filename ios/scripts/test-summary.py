"""Prints the outcome of an iOS test run in one line, plus any failures.

Reads the JSON that `xcrun xcresulttool get test-results summary` writes for a
result bundle. `xcodebuild -quiet` prints nothing when every test passes, so
without this a passing run looks the same as one that never started.
"""
import json
import sys

summary = json.load(sys.stdin)
print(
    f"{summary['result']}: {summary['passedTests']} passed, "
    f"{summary['failedTests']} failed, {summary['skippedTests']} skipped"
)
for failure in summary.get("testFailures", []):
    print(f"  {failure.get('testName')}: {failure.get('failureText')}")
