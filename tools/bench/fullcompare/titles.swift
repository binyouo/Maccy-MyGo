// Normalize the shared corpus exactly as ordinary persisted Swift titles:
// Swift Characters (graphemes), 1000-character bound, no special-symbol display.
import Foundation
let args = CommandLine.arguments
let titles = try JSONDecoder().decode([String].self, from: Data(contentsOf: URL(fileURLWithPath: args[1])))
let normalized = titles.map { String($0.prefix(1000)).replacingOccurrences(of: "\u{fffc}", with: "").trimmingCharacters(in: .whitespacesAndNewlines) }
try JSONEncoder().encode(normalized).write(to: URL(fileURLWithPath: args[2]))
