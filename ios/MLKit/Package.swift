// swift-tools-version: 6.0
import PackageDescription

// Google's ML Kit translator, the one Android uses, as one library the app links: `import MLKitTranslate`.
//
// Google publishes ML Kit for iOS only for CocoaPods, as pre-built frameworks. `ios/scripts/fetch-mlkit.sh`
// (`make ios-mlkit`) downloads them from Google into `Frameworks/`, which git ignores, and this package links
// them together with the open-source libraries their code calls: from those libraries' own Swift packages, and
// GoogleToolboxForMac built from source by the same script. The versions are the ones Google's podspecs ask for.
// ADR 0024.
let package = Package(
    name: "MLKit",
    // The app's minimum. ML Kit itself needs 15.5.
    platforms: [.iOS(.v18)],
    products: [
        .library(
            name: "MLKitTranslate",
            targets: ["MLKitTranslate", "MLKitNaturalLanguage", "MLKitCommon", "GoogleToolboxForMac", "MLKitSupport"]
        ),
    ],
    dependencies: [
        .package(url: "https://github.com/google/gtm-session-fetcher.git", "3.3.2"..<"4.0.0"),
        .package(url: "https://github.com/google/GoogleDataTransport.git", "10.0.0"..<"11.0.0"),
        .package(url: "https://github.com/google/GoogleUtilities.git", "8.0.0"..<"9.0.0"),
        .package(url: "https://github.com/ZipArchive/ZipArchive.git", "2.5.5"..<"3.0.0"),
    ],
    targets: [
        .binaryTarget(name: "MLKitTranslate", path: "Frameworks/MLKitTranslate.xcframework"),
        .binaryTarget(name: "MLKitNaturalLanguage", path: "Frameworks/MLKitNaturalLanguage.xcframework"),
        .binaryTarget(name: "MLKitCommon", path: "Frameworks/MLKitCommon.xcframework"),
        // Logger, StringEncoding and NSData+zlib from 4.2.1, built by the script: version 4 has no Swift package,
        // and the later package cannot be linked by Xcode (one of its parts is headers only).
        .binaryTarget(name: "GoogleToolboxForMac", path: "Frameworks/GoogleToolboxForMac.xcframework"),
        // No code of its own: it carries what Google's frameworks need at link time, since a binary target
        // cannot have dependencies.
        .target(
            name: "MLKitSupport",
            dependencies: [
                .product(name: "GTMSessionFetcherCore", package: "gtm-session-fetcher"),
                .product(name: "GoogleDataTransport", package: "GoogleDataTransport"),
                .product(name: "GULLogger", package: "GoogleUtilities"),
                .product(name: "GULUserDefaults", package: "GoogleUtilities"),
                .product(name: "ZipArchive", package: "ZipArchive"),
            ],
            linkerSettings: [
                // Most of Google's frameworks are archives in which some files hold only Objective-C
                // categories. Nothing refers to those by name, so without -ObjC the linker would leave them out
                // and their methods would be missing at run time. CocoaPods sets the same flag.
                .unsafeFlags(["-ObjC"]),
                // From the podspecs; the frameworks' module maps name the rest.
                .linkedLibrary("c++"),
                .linkedLibrary("sqlite3"),
                .linkedLibrary("z"),
                .linkedFramework("LocalAuthentication"),
            ]
        ),
    ]
)
