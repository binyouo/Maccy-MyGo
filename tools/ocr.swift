import Foundation
import Vision

guard CommandLine.arguments.count > 1 else { exit(2) }
let url = URL(fileURLWithPath: CommandLine.arguments[1])
let handler = VNImageRequestHandler(url: url)
let request = VNRecognizeTextRequest()
request.recognitionLevel = .accurate
if #available(macOS 13.0, *) {
    request.automaticallyDetectsLanguage = true
}
do {
    try handler.perform([request])
    if let results = request.results {
        var lines: [String] = []
        for obs in results {
            if let top = obs.topCandidates(1).first { lines.append(top.string) }
        }
        print(lines.joined(separator: "\n"))
    }
} catch {
    FileHandle.standardError.write("ocr: \(error)\n".data(using: .utf8)!)
    exit(1)
}
