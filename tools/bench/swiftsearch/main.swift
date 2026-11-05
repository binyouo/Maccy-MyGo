// search.swift mirrors Maccy's Search.swift (exact / regexp / fuzzy / mixed)
// so its cost can be measured against the Go port on the same corpus.
// Compile together with fuse-swift's Classes (Fuse.swift, FuseUtilities.swift,
// String+Fuse.swift), e.g.:
//
//	swiftc -O -o searchbench-swift tools/bench/search.swift \
//	  /path/to/fuse-swift/Fuse/Classes/*.swift
//
// Args: <corpus.json> <queries.json> [n] [reps] [warmup] [modes]

import Foundation

struct Query: Codable { let name: String; let query: String }
struct Result: Codable {
  let mode: String
  let query_name: String
  let matches: Int
  let min_ns: UInt64
  let median_ns: UInt64
  let p95_ns: UInt64
}

let fuzzySearchLimit = 5_000
let fuse = Fuse(threshold: 0.7) // Maccy: threshold found by trial-and-error

// --- Maccy Search.swift logic, Searchable reduced to a plain title String ---

func simpleSearch(_ q: String, _ items: [String], _ opts: String.CompareOptions) -> Int {
  var matches = 0
  for t in items {
    if t.range(of: q, options: opts, range: nil, locale: nil) != nil { matches += 1 }
  }
  return matches
}

func fuzzySearch(_ q: String, _ items: [String]) -> Int {
  let pattern = fuse.createPattern(from: q)
  var results: [(score: Double, ranges: Int)] = []
  results.reserveCapacity(64)
  for raw in items {
    var t = raw
    if t.count > fuzzySearchLimit {
      let stop = t.index(t.startIndex, offsetBy: fuzzySearchLimit)
      t = String(t[...stop])
    }
    if let found = fuse.search(pattern, in: t) {
      // Same String.index arithmetic Maccy pays to map Fuse ranges.
      let mapped = found.ranges.map { r -> Range<String.Index> in
        let s = t.index(t.startIndex, offsetBy: r.lowerBound)
        let e = t.index(t.startIndex, offsetBy: r.upperBound + 1)
        return s..<e
      }
      results.append((found.score, mapped.count))
    }
  }
  results.sort { $0.score < $1.score }
  return results.count
}

func runSearch(_ mode: String, _ q: String, _ items: [String]) -> Int {
  switch mode {
  case "exact": return simpleSearch(q, items, .caseInsensitive)
  case "regexp": return simpleSearch(q, items, .regularExpression)
  case "fuzzy": return fuzzySearch(q, items)
  case "mixed":
    var r = simpleSearch(q, items, .caseInsensitive)
    guard r == 0 else { return r }
    r = simpleSearch(q, items, .regularExpression)
    guard r == 0 else { return r }
    return fuzzySearch(q, items)
  default: return 0
  }
}

// --- harness ---

func readJSON<T: Decodable>(_ path: String, _ t: T.Type) -> T {
  let data = try! Data(contentsOf: URL(fileURLWithPath: path))
  return try! JSONDecoder().decode(t, from: data)
}

let args = CommandLine.arguments
var titles = readJSON(args[1], [String].self)
let queries = readJSON(args[2], [Query].self)
let n = args.count > 3 ? Int(args[3]) ?? 0 : 0
let reps = args.count > 4 ? Int(args[4]) ?? 20 : 20
let warmup = args.count > 5 ? Int(args[5]) ?? 3 : 3
let modes = (args.count > 6 ? args[6] : "exact,regexp,fuzzy,mixed").split(separator: ",").map(String.init)
if n > 0 && n < titles.count { titles = Array(titles.prefix(n)) }

var out: [Result] = []
for mode in modes {
  for q in queries {
    for _ in 0..<warmup { _ = runSearch(mode, q.query, titles) }
    var ds = [UInt64]()
    ds.reserveCapacity(reps)
    var matches = 0
    for _ in 0..<reps {
      let t0 = DispatchTime.now().uptimeNanoseconds
      matches = runSearch(mode, q.query, titles)
      ds.append(DispatchTime.now().uptimeNanoseconds - t0)
    }
    ds.sort()
    out.append(Result(
      mode: mode, query_name: q.name, matches: matches,
      min_ns: ds[0], median_ns: ds[ds.count / 2], p95_ns: ds[ds.count * 19 / 20]))
  }
}

let enc = JSONEncoder()
enc.outputFormatting = [.prettyPrinted]
FileHandle.standardOutput.write(try! enc.encode(out))
FileHandle.standardError.write("corpus=\(titles.count) queries=\(queries.count) reps=\(reps)\n".data(using: .utf8)!)
