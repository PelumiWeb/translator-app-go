// Draws the app icon and the launch screen's mark into the asset catalog, from the logo's geometry in
// docs/design/pearl/README.md ("Logo and splash"). Run it again if the logo changes:
//
//     swift ios/scripts/render-brand.swift
//
// Drawn here rather than converted from the SVGs, since converting needs a tool this Mac does not have.
import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

let assets = URL(fileURLWithPath: CommandLine.arguments[0])
    .deletingLastPathComponent().deletingLastPathComponent()
    .appendingPathComponent("PTranslate/Assets.xcassets")

func color(_ hex: UInt32, alpha: CGFloat = 1) -> CGColor {
    CGColor(
        srgbRed: CGFloat((hex >> 16) & 0xFF) / 255,
        green: CGFloat((hex >> 8) & 0xFF) / 255,
        blue: CGFloat(hex & 0xFF) / 255,
        alpha: alpha
    )
}

let petrol = color(0x0E2426)
let mint = color(0x7FD1C1)
let warmWhite = color(0xF3EEE4)
let lens = color(0xB9E0D3)

/// The mark on petrol, in the logo's own units: circles of radius 22 at (46, 40) and (74, 40), their overlap in its
/// own colour, and a horizon line at y = 62. The context is already scaled and moved to those units.
func drawMark(_ ctx: CGContext) {
    ctx.setStrokeColor(color(0xF3EEE4, alpha: 0.5))
    ctx.setLineWidth(2.5)
    ctx.setLineCap(.round)
    ctx.strokeLineSegments(between: [CGPoint(x: 8, y: 62), CGPoint(x: 112, y: 62)])

    let left = CGRect(x: 46 - 22, y: 40 - 22, width: 44, height: 44)
    let right = CGRect(x: 74 - 22, y: 40 - 22, width: 44, height: 44)
    ctx.setFillColor(mint)
    ctx.fillEllipse(in: left)
    ctx.setFillColor(warmWhite)
    ctx.fillEllipse(in: right)
    ctx.saveGState()
    ctx.addEllipse(in: left)
    ctx.clip()
    ctx.setFillColor(lens)
    ctx.fillEllipse(in: right)
    ctx.restoreGState()
}

/// A bitmap `width` by `height` pixels, drawn with y pointing down as in the SVGs. An opaque one has no alpha
/// channel, which the App Store requires of the icon.
func render(width: Int, height: Int, opaque: Bool, to path: String, draw: (CGContext) -> Void) throws {
    let info = opaque ? CGImageAlphaInfo.noneSkipLast : CGImageAlphaInfo.premultipliedLast
    guard let ctx = CGContext(
        data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
        space: CGColorSpace(name: CGColorSpace.sRGB)!, bitmapInfo: info.rawValue
    ) else { throw CocoaError(.fileWriteUnknown) }
    ctx.translateBy(x: 0, y: CGFloat(height))
    ctx.scaleBy(x: 1, y: -1)
    draw(ctx)

    let url = assets.appendingPathComponent(path)
    guard let image = ctx.makeImage(),
          let file = CGImageDestinationCreateWithURL(url as CFURL, UTType.png.identifier as CFString, 1, nil)
    else { throw CocoaError(.fileWriteUnknown) }
    CGImageDestinationAddImage(file, image, nil)
    guard CGImageDestinationFinalize(file) else { throw CocoaError(.fileWriteUnknown) }
    print("wrote \(path)")
}

// The icon: full bleed petrol, the mark centred at the scale of shinar-app-icon-1024.svg. iOS rounds the corners.
try render(width: 1024, height: 1024, opaque: true, to: "AppIcon.appiconset/AppIcon.png") { ctx in
    ctx.setFillColor(petrol)
    ctx.fill(CGRect(x: 0, y: 0, width: 1024, height: 1024))
    ctx.translateBy(x: 512, y: 512)
    ctx.scaleBy(x: 5.9, y: 5.9)
    ctx.translateBy(x: -60, y: -41)
    drawMark(ctx)
}

// The launch screen's mark: 135 points wide (the logo's 108 units), centred by iOS on the petrol background. The
// canvas is 63 points tall so it is whole pixels at 2x and 3x.
for scale in [2, 3] {
    try render(width: 135 * scale, height: 63 * scale, opaque: false, to: "LaunchMark.imageset/LaunchMark@\(scale)x.png") { ctx in
        let unit = 135.0 / 108.0 * CGFloat(scale)
        ctx.translateBy(x: 0, y: 0.25 * CGFloat(scale))
        ctx.scaleBy(x: unit, y: unit)
        ctx.translateBy(x: -6, y: -16)
        drawMark(ctx)
    }
}
