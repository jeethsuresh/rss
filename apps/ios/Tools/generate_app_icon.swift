import CoreGraphics
import Foundation
import ImageIO
import UniformTypeIdentifiers

let side = 1024
let colorSpace = CGColorSpaceCreateDeviceRGB()
guard let context = CGContext(
    data: nil,
    width: side,
    height: side,
    bitsPerComponent: 8,
    bytesPerRow: side * 4,
    space: colorSpace,
    bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue
) else {
    fatalError("Could not create icon canvas")
}

func color(_ red: CGFloat, _ green: CGFloat, _ blue: CGFloat, _ alpha: CGFloat = 1) -> CGColor {
    CGColor(colorSpace: colorSpace, components: [red, green, blue, alpha])!
}

let bounds = CGRect(x: 0, y: 0, width: side, height: side)
let background = CGGradient(
    colorsSpace: colorSpace,
    colors: [color(0.035, 0.055, 0.13), color(0.075, 0.055, 0.20)] as CFArray,
    locations: [0, 1]
)!
context.drawLinearGradient(
    background,
    start: CGPoint(x: 100, y: 100),
    end: CGPoint(x: 920, y: 920),
    options: [.drawsBeforeStartLocation, .drawsAfterEndLocation]
)

context.saveGState()
context.setShadow(offset: CGSize(width: 0, height: -24), blur: 38, color: color(0, 0, 0, 0.28))
let page = CGPath(roundedRect: CGRect(x: 148, y: 160, width: 560, height: 704), cornerWidth: 82, cornerHeight: 82, transform: nil)
context.addPath(page)
context.setFillColor(color(0.98, 0.965, 0.91))
context.fillPath()
context.restoreGState()

let photo = CGPath(roundedRect: CGRect(x: 230, y: 644, width: 290, height: 116), cornerWidth: 28, cornerHeight: 28, transform: nil)
context.addPath(photo)
context.setFillColor(color(1.0, 0.31, 0.28))
context.fillPath()

context.setStrokeColor(color(0.17, 0.17, 0.25, 0.62))
context.setLineCap(.round)
context.setLineWidth(30)
for y in [570, 502, 434, 366] as [CGFloat] {
    context.move(to: CGPoint(x: 230, y: y))
    context.addLine(to: CGPoint(x: y == 366 ? 430 : 520, y: y))
    context.strokePath()
}

let accent = color(1.0, 0.52, 0.12)
context.setStrokeColor(accent)
context.setLineCap(.round)
for (radius, width) in [(170.0, 64.0), (290.0, 70.0), (414.0, 76.0)] {
    context.setLineWidth(width)
    context.addArc(
        center: CGPoint(x: 548, y: 310),
        radius: radius,
        startAngle: 0.10,
        endAngle: 1.47,
        clockwise: false
    )
    context.strokePath()
}

context.setFillColor(accent)
context.fillEllipse(in: CGRect(x: 492, y: 254, width: 112, height: 112))

context.setStrokeColor(color(1, 1, 1, 0.20))
context.setLineWidth(5)
context.addArc(center: CGPoint(x: 548, y: 310), radius: 414, startAngle: 0.10, endAngle: 1.47, clockwise: false)
context.strokePath()

guard let image = context.makeImage() else { fatalError("Could not render icon") }
let destinationURL = URL(fileURLWithPath: CommandLine.arguments[1]) as CFURL
guard let destination = CGImageDestinationCreateWithURL(destinationURL, UTType.png.identifier as CFString, 1, nil) else {
    fatalError("Could not create PNG destination")
}
CGImageDestinationAddImage(destination, image, nil)
guard CGImageDestinationFinalize(destination) else { fatalError("Could not write PNG") }
