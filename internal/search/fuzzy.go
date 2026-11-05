package search

// Adapted from Fuse 1.4.0's Bitap search (MIT), revision
// 26ba868691b2d8b7bf2b1322951eb591be70ccca, the version in Maccy's
// Package.resolved. See LICENSE.fuse-swift for the retained notice.

import "math"

type fuzzyPattern struct {
	text     []string
	mask     uint64
	alphabet map[string]uint64
}

func bit(n int) uint64 {
	if n < 0 || n >= 64 {
		return 0
	}
	return uint64(1) << uint(n)
}

func makeFuzzyPattern(query string) fuzzyPattern {
	p := fuzzyPattern{text: characters(query, true, 0).text, alphabet: map[string]uint64{}}
	p.mask = bit(len(p.text) - 1)
	for i, c := range p.text {
		p.alphabet[c] |= bit(len(p.text) - i - 1)
	}
	return p
}

func fuzzyScore(length, errors, location int) float64 {
	return float64(errors)/float64(length) + math.Abs(float64(location))/100
}

// fuzzyScratch carries the working buffers reused across match calls so a
// full-table scan does not allocate per item (mask) or per error level
// (bitap rows).
type fuzzyScratch struct {
	mask []bool
	u1   []uint64
	u2   []uint64
}

func (s *fuzzyScratch) maskBuf(n int) []bool {
	if cap(s.mask) < n {
		s.mask = make([]bool, n)
		return s.mask
	}
	clear(s.mask[:n])
	return s.mask[:n]
}

func (s *fuzzyScratch) growBits(n int) {
	if cap(s.u1) < n {
		s.u1 = make([]uint64, n)
	}
	if cap(s.u2) < n {
		s.u2 = make([]uint64, n)
	}
}

func (p fuzzyPattern) match(title string, sc *fuzzyScratch) ([]Range, float64, bool) {
	chars := characters(title, true, 5001)
	text := chars.text
	n, m := len(text), len(p.text)
	if m == 0 {
		return nil, 0, false
	}
	same := n == m
	if same {
		for i := range text {
			if text[i] != p.text[i] {
				same = false
				break
			}
		}
	}
	if same {
		return []Range{{0, chars.offsets[n]}}, 0, true
	}
	// Swift's Int-sized pattern masks cannot represent a longer pattern.
	// Return no match safely instead of reaching overflowing Swift shifts.
	if m > 64 {
		return nil, 0, false
	}
	threshold := 0.7
	matchMask := sc.maskBuf(n)
	for start := 0; start+m <= n; {
		index := -1
		for i := start; i+m <= n; i++ {
			found := true
			for j := range p.text {
				if text[i+j] != p.text[j] {
					found = false
					break
				}
			}
			if found {
				index = i
				break
			}
		}
		if index < 0 {
			break
		}
		threshold = math.Min(threshold, fuzzyScore(m, 0, index))
		for j := 0; j < m; j++ {
			matchMask[index+j] = true
		}
		start = index + m
	}
	score := 1.0
	binMax := m + n
	sc.growBits(n + m + 2)
	b1, b2 := sc.u1, sc.u2
	var last []uint64
	flip := false
	for errors := 0; errors < m; errors++ {
		lo, mid := 0, binMax
		for lo < mid {
			if fuzzyScore(m, errors, mid) <= threshold {
				lo = mid
			} else {
				binMax = mid
			}
			mid = (binMax-lo)/2 + lo
		}
		binMax = mid
		finish := min(mid, n) + m
		bits := b1[:finish+2]
		if flip {
			bits = b2[:finish+2]
		}
		bits[finish+1] = bit(errors) - 1
		for j := finish; j >= 1; j-- {
			location := j - 1
			var charMatch uint64
			if location < n {
				charMatch = p.alphabet[text[location]]
			}
			if charMatch != 0 {
				matchMask[location] = true
			}
			bits[j] = ((bits[j+1] << 1) | 1) & charMatch
			if errors > 0 {
				bits[j] |= (((last[j+1] | last[j]) << 1) | 1) | last[j+1]
			}
			if bits[j]&p.mask != 0 {
				score = fuzzyScore(m, errors, location)
				if score <= threshold {
					threshold = score
					if location == 0 {
						break
					}
				}
			}
		}
		if fuzzyScore(m, errors+1, 0) > threshold {
			break
		}
		last = bits
		flip = !flip
	}
	if score == 1 {
		return nil, 0, false
	}
	var ranges []Range
	for i := 0; i < n; {
		if !matchMask[i] {
			i++
			continue
		}
		start := i
		for i < n && matchMask[i] {
			i++
		}
		ranges = append(ranges, Range{chars.offsets[start], chars.offsets[i]})
	}
	return ranges, score, true
}
