import AppKit
let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: 1200, pixelsHigh: 360, bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false, colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: bitmap)
NSColor.white.setFill(); NSBezierPath(rect: NSRect(x:0,y:0,width:1200,height:360)).fill()
("FullCompare OCR 2026" as NSString).draw(at:NSPoint(x:60,y:230),withAttributes:[.font:NSFont.systemFont(ofSize:56),.foregroundColor:NSColor.black])
("剪贴板性能对比" as NSString).draw(at:NSPoint(x:60,y:110),withAttributes:[.font:NSFont.systemFont(ofSize:56),.foregroundColor:NSColor.black])
NSGraphicsContext.restoreGraphicsState()
try bitmap.representation(using:.png,properties:[:])!.write(to:URL(fileURLWithPath:CommandLine.arguments[1]))
