// Copyright 2021 The LevelDB-Go and Pebble Authors. All rights reserved. Use
// of this source code is governed by a BSD-style license that can be found in
// the LICENSE file.

// Package testkeys provides facilities for generating and comparing
// human-readable test keys for use in tests and benchmarks. This package
// provides a single Comparer implementation that compares all keys generated
// by this package.
//
// Keys generated by this package may optionally have a 'suffix' encoding an
// MVCC timestamp. This suffix is of the form "@<integer>". Comparisons on the
// suffix are performed using integer value, not the byte representation.
package testkeys

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/cockroachdb/pebble/internal/base"
	"golang.org/x/exp/constraints"
	"golang.org/x/exp/rand"
)

const alpha = "abcdefghijklmnopqrstuvwxyz"

const suffixDelim = '@'

var inverseAlphabet = make(map[byte]int64, len(alpha))

func init() {
	for i := range alpha {
		inverseAlphabet[alpha[i]] = int64(i)
	}
}

// MaxSuffixLen is the maximum length of a suffix generated by this package.
var MaxSuffixLen = 1 + len(fmt.Sprintf("%d", int64(math.MaxInt64)))

// Comparer is the comparer for test keys generated by this package.
var Comparer = &base.Comparer{
	Compare: compare,
	Equal:   func(a, b []byte) bool { return compare(a, b) == 0 },
	AbbreviatedKey: func(k []byte) uint64 {
		return base.DefaultComparer.AbbreviatedKey(k[:split(k)])
	},
	FormatKey: base.DefaultFormatter,
	Separator: func(dst, a, b []byte) []byte {
		ai := split(a)
		if ai == len(a) {
			return append(dst, a...)
		}
		bi := split(b)
		if bi == len(b) {
			return append(dst, a...)
		}

		// If the keys are the same just return a.
		if bytes.Equal(a[:ai], b[:bi]) {
			return append(dst, a...)
		}
		n := len(dst)
		dst = base.DefaultComparer.Separator(dst, a[:ai], b[:bi])
		// Did it pick a separator different than a[:ai] -- if not we can't do better than a.
		buf := dst[n:]
		if bytes.Equal(a[:ai], buf) {
			return append(dst[:n], a...)
		}
		// The separator is > a[:ai], so return it
		return dst
	},
	Successor: func(dst, a []byte) []byte {
		ai := split(a)
		if ai == len(a) {
			return append(dst, a...)
		}
		n := len(dst)
		dst = base.DefaultComparer.Successor(dst, a[:ai])
		// Did it pick a successor different than a[:ai] -- if not we can't do better than a.
		buf := dst[n:]
		if bytes.Equal(a[:ai], buf) {
			return append(dst[:n], a...)
		}
		// The successor is > a[:ai], so return it.
		return dst
	},
	ImmediateSuccessor: func(dst, a []byte) []byte {
		// TODO(jackson): Consider changing this Comparer to only support
		// representable prefix keys containing characters a-z.
		ai := split(a)
		if ai != len(a) {
			panic("pebble: ImmediateSuccessor invoked with a non-prefix key")
		}
		return append(append(dst, a...), 0x00)
	},
	Split: split,
	Name:  "pebble.internal.testkeys",
}

func compare(a, b []byte) int {
	ai, bi := split(a), split(b)
	if v := bytes.Compare(a[:ai], b[:bi]); v != 0 {
		return v
	}

	if len(a[ai:]) == 0 {
		if len(b[bi:]) == 0 {
			return 0
		}
		return -1
	} else if len(b[bi:]) == 0 {
		return +1
	}
	return compareTimestamps(a[ai:], b[bi:])
}

func split(a []byte) int {
	i := bytes.LastIndexByte(a, suffixDelim)
	if i >= 0 {
		return i
	}
	return len(a)
}

func compareTimestamps(a, b []byte) int {
	ai, err := parseUintBytes(bytes.TrimPrefix(a, []byte{suffixDelim}), 10, 64)
	if err != nil {
		panic(fmt.Sprintf("invalid test mvcc timestamp %q", a))
	}
	bi, err := parseUintBytes(bytes.TrimPrefix(b, []byte{suffixDelim}), 10, 64)
	if err != nil {
		panic(fmt.Sprintf("invalid test mvcc timestamp %q", b))
	}
	switch {
	case ai < bi:
		return +1
	case ai > bi:
		return -1
	default:
		return 0
	}
}

// Keyspace describes a finite keyspace of unsuffixed test keys.
type Keyspace interface {
	// Count returns the number of keys that exist within this keyspace.
	Count() int64

	// MaxLen returns the maximum length, in bytes, of a key within this
	// keyspace. This is only guaranteed to return an upper bound.
	MaxLen() int

	// Slice returns the sub-keyspace from index i, inclusive, to index j,
	// exclusive. The receiver is unmodified.
	Slice(i, j int64) Keyspace

	// EveryN returns a key space that includes 1 key for every N keys in the
	// original keyspace. The receiver is unmodified.
	EveryN(n int64) Keyspace

	// key writes the i-th key to the buffer and returns the length.
	key(buf []byte, i int64) int
}

// Divvy divides the provided keyspace into N equal portions, containing
// disjoint keys evenly distributed across the keyspace.
func Divvy(ks Keyspace, n int64) []Keyspace {
	ret := make([]Keyspace, n)
	for i := int64(0); i < n; i++ {
		ret[i] = ks.Slice(i, ks.Count()).EveryN(n)
	}
	return ret
}

// Alpha constructs a keyspace consisting of all keys containing characters a-z,
// with at most `maxLength` characters.
func Alpha(maxLength int) Keyspace {
	return alphabet{
		alphabet:  []byte(alpha),
		maxLength: maxLength,
		increment: 1,
	}
}

// KeyAt returns the i-th key within the keyspace with a suffix encoding the
// timestamp t.
func KeyAt(k Keyspace, i int64, t int64) []byte {
	b := make([]byte, k.MaxLen()+MaxSuffixLen)
	return b[:WriteKeyAt(b, k, i, t)]
}

// WriteKeyAt writes the i-th key within the keyspace to the buffer dst, with a
// suffix encoding the timestamp t suffix. It returns the number of bytes
// written.
func WriteKeyAt(dst []byte, k Keyspace, i int64, t int64) int {
	n := WriteKey(dst, k, i)
	n += WriteSuffix(dst[n:], t)
	return n
}

// Suffix returns the test keys suffix representation of timestamp t.
func Suffix(t int64) []byte {
	b := make([]byte, MaxSuffixLen)
	return b[:WriteSuffix(b, t)]
}

// SuffixLen returns the exact length of the given suffix when encoded.
func SuffixLen(t int64) int {
	// Begin at 1 for the '@' delimiter, 1 for a single digit.
	n := 2
	t /= 10
	for t > 0 {
		t /= 10
		n++
	}
	return n
}

// ParseSuffix returns the integer representation of the encoded suffix.
func ParseSuffix(s []byte) (int64, error) {
	return strconv.ParseInt(strings.TrimPrefix(string(s), string(suffixDelim)), 10, 64)
}

// WriteSuffix writes the test keys suffix representation of timestamp t to dst,
// returning the number of bytes written.
func WriteSuffix(dst []byte, t int64) int {
	dst[0] = suffixDelim
	n := 1
	n += len(strconv.AppendInt(dst[n:n], t, 10))
	return n
}

// Key returns the i-th unsuffixed key within the keyspace.
func Key(k Keyspace, i int64) []byte {
	b := make([]byte, k.MaxLen())
	return b[:k.key(b, i)]
}

// WriteKey writes the i-th unsuffixed key within the keyspace to the buffer dst. It
// returns the number of bytes written.
func WriteKey(dst []byte, k Keyspace, i int64) int {
	return k.key(dst, i)
}

type alphabet struct {
	alphabet  []byte
	maxLength int
	headSkip  int64
	tailSkip  int64
	increment int64
}

func (a alphabet) Count() int64 {
	// Calculate the total number of keys, ignoring the increment.
	total := keyCount(len(a.alphabet), a.maxLength) - a.headSkip - a.tailSkip

	// The increment dictates that we take every N keys, where N = a.increment.
	// Consider a total containing the 5 keys:
	//   a  b  c  d  e
	//   ^     ^     ^
	// If the increment is 2, this keyspace includes 'a', 'c' and 'e'. After
	// dividing by the increment, there may be remainder. If there is, there's
	// one additional key in the alphabet.
	count := total / a.increment
	if total%a.increment > 0 {
		count++
	}
	return count
}

func (a alphabet) MaxLen() int {
	return a.maxLength
}

func (a alphabet) Slice(i, j int64) Keyspace {
	s := a
	s.headSkip += i
	s.tailSkip += a.Count() - j
	return s
}

func (a alphabet) EveryN(n int64) Keyspace {
	s := a
	s.increment *= n
	return s
}

func keyCount(n, l int) int64 {
	if n == 0 {
		return 0
	} else if n == 1 {
		return int64(l)
	}
	// The number of representable keys in the keyspace is a function of the
	// length of the alphabet n and the max key length l. Consider how the
	// number of representable keys grows as l increases:
	//
	// l = 1: n
	// l = 2: n + n^2
	// l = 3: n + n^2 + n^3
	// ...
	// Σ i=(1...l) n^i = n*(n^l - 1)/(n-1)
	return (int64(n) * (int64(math.Pow(float64(n), float64(l))) - 1)) / int64(n-1)
}

func (a alphabet) key(buf []byte, idx int64) int {
	// This function generates keys of length 1..maxKeyLength, pulling
	// characters from the alphabet. The idx determines which key to generate,
	// generating the i-th lexicographically next key.
	//
	// The index to use is advanced by `headSkip`, allowing a keyspace to encode
	// a subregion of the keyspace.
	//
	// Eg, alphabet = `ab`, maxKeyLength = 3:
	//
	//           aaa aab     aba abb         baa bab     bba bbb
	//       aa          ab              ba          bb
	//   a                           b
	//   0   1   2   3   4   5   6   7   8   9   10  11  12  13
	//
	return generateAlphabetKey(buf, a.alphabet, (idx*a.increment)+a.headSkip,
		keyCount(len(a.alphabet), a.maxLength))
}

func generateAlphabetKey(buf, alphabet []byte, i, keyCount int64) int {
	if keyCount == 0 || i > keyCount || i < 0 {
		return 0
	}

	// Of the keyCount keys in the generative keyspace, how many are there
	// starting with a particular character?
	keysPerCharacter := keyCount / int64(len(alphabet))

	// Find the character that the key at index i starts with and set it.
	characterIdx := i / keysPerCharacter
	buf[0] = alphabet[characterIdx]

	// Consider characterIdx = 0, pointing to 'a'.
	//
	//           aaa aab     aba abb         baa bab     bba bbb
	//       aa          ab              ba          bb
	//   a                           b
	//   0   1   2   3   4   5   6   7   8   9   10  11  12  13
	//  \_________________________/
	//    |keysPerCharacter| keys
	//
	// In our recursive call, we reduce the problem to:
	//
	//           aaa aab     aba abb
	//       aa          ab
	//       0   1   2   3   4   5
	//     \________________________/
	//    |keysPerCharacter-1| keys
	//
	// In the subproblem, there are keysPerCharacter-1 keys (eliminating the
	// just 'a' key, plus any keys beginning with any other character).
	//
	// The index i is also offset, reduced by the count of keys beginning with
	// characters earlier in the alphabet (keysPerCharacter*characterIdx) and
	// the key consisting of just the 'a' (-1).
	i = i - keysPerCharacter*characterIdx - 1
	return 1 + generateAlphabetKey(buf[1:], alphabet, i, keysPerCharacter-1)
}

// computeAlphabetKeyIndex computes the inverse of generateAlphabetKey,
// returning the index of a particular key, given the provided alphabet and max
// length of a key.
//
// len(key) must be ≥ 1.
func computeAlphabetKeyIndex(key []byte, alphabet map[byte]int64, n int) int64 {
	i, ok := alphabet[key[0]]
	if !ok {
		panic(fmt.Sprintf("unrecognized alphabet character %v", key[0]))
	}
	// How many keys exist that start with the preceding i characters? Each of
	// the i characters themselves are a key, plus the count of all the keys
	// with one less character for each.
	ret := i + i*keyCount(len(alphabet), n-1)
	if len(key) > 1 {
		ret += 1 + computeAlphabetKeyIndex(key[1:], alphabet, n-1)
	}
	return ret
}

func abs(a int64) int64 {
	if a < 0 {
		return -a
	}
	return a
}

// RandomSeparator returns a random alphabetic key k such that a < k < b,
// pulling randomness from the provided random number generator. If dst is
// provided and the generated key fits within dst's capacity, the returned slice
// will use dst's memory.
//
// If a prefix P exists such that Prefix(a) < P < Prefix(b), the generated key
// will consist of the prefix P appended with the provided suffix. A zero suffix
// generates an unsuffixed key. If no such prefix P exists, RandomSeparator will
// try to find a key k with either Prefix(a) or Prefix(b) such that a < k < b,
// but the generated key will not use the provided suffix. Note that it's
// possible that no separator key exists (eg, a='a@2', b='a@1'), in which case
// RandomSeparator returns nil.
//
// If RandomSeparator generates a new prefix, the generated prefix will have
// length at most MAX(maxLength, len(Prefix(a)), len(Prefix(b))).
//
// RandomSeparator panics if a or b fails to decode.
func RandomSeparator(dst, a, b []byte, suffix int64, maxLength int, rng *rand.Rand) []byte {
	if Comparer.Compare(a, b) >= 0 {
		return nil
	}

	// Determine both keys' logical prefixes and suffixes.
	ai := Comparer.Split(a)
	bi := Comparer.Split(b)
	ap := a[:ai]
	bp := b[:bi]
	maxLength = max[int](maxLength, max[int](len(ap), len(bp)))
	var as, bs int64
	var err error
	if ai != len(a) {
		as, err = ParseSuffix(a[ai:])
		if err != nil {
			panic(fmt.Sprintf("failed to parse suffix of %q", a))
		}
	}
	if bi != len(b) {
		bs, err = ParseSuffix(b[bi:])
		if err != nil {
			panic(fmt.Sprintf("failed to parse suffix of %q", b))
		}
	}

	apIdx := computeAlphabetKeyIndex(ap, inverseAlphabet, maxLength)
	bpIdx := computeAlphabetKeyIndex(bp, inverseAlphabet, maxLength)
	diff := bpIdx - apIdx
	generatedIdx := bpIdx
	if diff > 0 {
		var add int64 = diff + 1
		var start int64 = apIdx
		if as == 1 {
			// There's no expressible key with prefix a greater than a@1. So,
			// exclude ap.
			start = apIdx + 1
			add = diff
		}
		if bs == 0 {
			// No key with prefix b can sort before b@0. We don't want to pick b.
			add--
		}
		// We're allowing generated id to be in the range [start, start + add - 1].
		if start > start+add-1 {
			return nil
		}
		// If we can generate a key which is actually in the middle of apIdx
		// and bpIdx use it so that we don't have to bother about timestamps.
		generatedIdx = rng.Int63n(add) + start
		for diff > 1 && generatedIdx == apIdx || generatedIdx == bpIdx {
			generatedIdx = rng.Int63n(add) + start
		}
	}

	switch {
	case generatedIdx == apIdx && generatedIdx == bpIdx:
		if abs(bs-as) <= 1 {
			// There's no expressible suffix between the two, and there's no
			// possible separator key.
			return nil
		}
		// The key b is >= key a, but has the same prefix, so b must have the
		// smaller timestamp, unless a has timestamp of 0.
		//
		// NB: The zero suffix (suffix-less) sorts before all other suffixes, so
		// any suffix we generate will be greater than it.
		if as == 0 {
			// bs > as
			suffix = bs + rng.Int63n(10) + 1
		} else {
			// bs < as.
			// Generate suffix in range [bs + 1, as - 1]
			suffix = bs + 1 + rng.Int63n(as-bs-1)
		}
	case generatedIdx == apIdx:
		// NB: The zero suffix (suffix-less) sorts before all other suffixes, so
		// any suffix we generate will be greater than it.
		if as == 0 && suffix == 0 {
			suffix++
		} else if as != 0 && suffix >= as {
			suffix = rng.Int63n(as)
		}
	case generatedIdx == bpIdx:
		if suffix <= bs {
			suffix = bs + rng.Int63n(10) + 1
		}
	}
	if sz := maxLength + SuffixLen(suffix); cap(dst) < sz {
		dst = make([]byte, sz)
	} else {
		dst = dst[:cap(dst)]
	}
	var w int
	if suffix == 0 {
		w = WriteKey(dst, Alpha(maxLength), generatedIdx)
	} else {
		w = WriteKeyAt(dst, Alpha(maxLength), generatedIdx, suffix)
	}
	return dst[:w]
}

func max[I constraints.Ordered](a, b I) I {
	if b > a {
		return b
	}
	return a
}
