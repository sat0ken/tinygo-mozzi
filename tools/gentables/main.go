// Command gentables converts Mozzi's Arduino wavetable headers
// (Mozzi/tables/*.h) into Go source, backed by //go:embed binary blobs.
//
// Rationale: unlike AVR, the RP2040 (and every other TinyGo-supported
// Cortex-M target) can address flash directly, so there is no need for
// Mozzi's PROGMEM/pgm_read_byte dance (see mozzi_pgmspace.h upstream).
// A plain Go slice pointed at an embedded, flash-resident byte blob
// gives the same "table lives in flash, not RAM" property for free.
//
// Usage:
//
//	go run ./tools/gentables <path-to-Mozzi/tables-dir> <output-package-dir>
//
// This is a build-time generator, not something sketches import; it is
// checked in mainly so the table set can be regenerated or extended
// (e.g. to pull in the BandLimited_* tables that were left out of the
// default port to keep flash usage modest -- see README).
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	reType    = regexp.MustCompile(`CONSTTABLE_STORAGE\(\s*(\w+)\s*\)\s*([A-Za-z_][A-Za-z0-9_]*)\s*\[[^\]]*\]\s*=`)
	reDefine  = regexp.MustCompile(`(?m)^#define\s+(\w+)_(NUM_CELLS|SAMPLERATE)\s+(\d+)`)
	reNumber  = regexp.MustCompile(`-?\d+`)
	reInvalid = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

type table struct {
	stem       string // e.g. "sin8192_int8" (source file name, minus .h)
	cType      string // int8_t, uint8_t, int16_t
	goName     string // e.g. Sin8192
	numCells   int
	sampleRate int // 0 if not defined
	values     []int64
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: gentables <Mozzi/tables dir> <output dir>")
		os.Exit(2)
	}
	srcDir, outDir := os.Args[1], os.Args[2]

	entries, err := os.ReadDir(srcDir)
	check(err)

	dataDir := filepath.Join(outDir, "tablesdata")
	check(os.MkdirAll(dataDir, 0o755))

	var names []string
	var docLines []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".h") {
			continue // skips BandLimited_SAW/SQUARE/TRI subdirectories on purpose
		}
		path := filepath.Join(srcDir, e.Name())
		src, err := os.ReadFile(path)
		check(err)

		t, err := parseTable(e.Name(), string(src))
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping %s: %v\n", e.Name(), err)
			continue
		}

		writeBin(filepath.Join(dataDir, t.stem+".bin"), t)
		writeGo(filepath.Join(outDir, t.stem+".go"), t)
		names = append(names, t.goName)
		docLines = append(docLines, fmt.Sprintf("//   - %s (%d cells%s) from tables/%s", t.goName, t.numCells, sampleRateSuffix(t), e.Name()))
	}

	sort.Strings(names)
	sort.Strings(docLines)
	writeDoc(filepath.Join(outDir, "doc.go"), docLines)

	fmt.Printf("generated %d tables into %s\n", len(names), outDir)
}

func sampleRateSuffix(t *table) string {
	if t.sampleRate == 0 {
		return ""
	}
	return fmt.Sprintf(", recorded at %dHz", t.sampleRate)
}

func parseTable(fileName, src string) (*table, error) {
	loc := reType.FindStringSubmatchIndex(src)
	if loc == nil {
		return nil, fmt.Errorf("no CONSTTABLE_STORAGE array found")
	}
	cType := src[loc[2]:loc[3]]
	arrName := src[loc[4]:loc[5]]
	matchEnd := loc[1]

	// Slice out the initializer body: from the first '{' after the match to
	// its matching '}'. These files never nest braces inside the array, so a
	// depth counter is enough.
	openIdx := strings.Index(src[matchEnd:], "{")
	if openIdx < 0 {
		return nil, fmt.Errorf("no '{' found after array declaration")
	}
	body := src[matchEnd+openIdx:]
	depth := 0
	end := -1
	for i, r := range body {
		switch r {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("unterminated array initializer")
	}
	numsStr := body[1:end]

	var values []int64
	for _, s := range reNumber.FindAllString(numsStr, -1) {
		v, err := strconv.ParseInt(s, 10, 64)
		check(err)
		values = append(values, v)
	}

	t := &table{
		cType:  cType,
		values: values,
		stem:   strings.TrimSuffix(fileName, ".h"),
		goName: goIdent(strings.TrimSuffix(arrName, "_DATA")),
	}

	for _, dm := range reDefine.FindAllStringSubmatch(src, -1) {
		n, _ := strconv.Atoi(dm[3])
		switch dm[2] {
		case "NUM_CELLS":
			t.numCells = n
		case "SAMPLERATE":
			t.sampleRate = n
		}
	}
	if t.numCells == 0 {
		t.numCells = len(values)
	}
	return t, nil
}

// goIdent turns e.g. "WAVESHAPE_SIGMOID" into "WaveshapeSigmoid".
func goIdent(s string) string {
	parts := strings.Split(strings.ToLower(s), "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	out := b.String()
	if out == "" || (out[0] >= '0' && out[0] <= '9') {
		out = "T" + out
	}
	return out
}

func writeBin(path string, t *table) {
	buf := make([]byte, 0, len(t.values)*2)
	for _, v := range t.values {
		switch t.cType {
		case "int8_t":
			buf = append(buf, byte(int8(v)))
		case "uint8_t":
			buf = append(buf, byte(uint8(v)))
		case "int16_t":
			u := uint16(int16(v))
			buf = append(buf, byte(u), byte(u>>8))
		case "uint16_t":
			u := uint16(v)
			buf = append(buf, byte(u), byte(u>>8))
		default:
			fatalf("unsupported C type %q in %s", t.cType, path)
		}
	}
	check(os.WriteFile(path, buf, 0o644))
}

func writeGo(path string, t *table) {
	var goType, convFunc string
	switch t.cType {
	case "int8_t":
		goType, convFunc = "int8", "asInt8"
	case "uint8_t":
		goType, convFunc = "uint8", "asUint8"
	case "int16_t":
		goType, convFunc = "int16", "asInt16LE"
	case "uint16_t":
		goType, convFunc = "uint16", "asUint16LE"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "// Code generated by tools/gentables from Mozzi/tables/%s.h; DO NOT EDIT.\n\n", t.stem)
	fmt.Fprintf(&b, "package tables\n\n")
	fmt.Fprintf(&b, "import _ \"embed\"\n\n")
	fmt.Fprintf(&b, "// %sNumCells is the number of cells in %s (always a power of two).\n", t.goName, t.goName)
	fmt.Fprintf(&b, "const %sNumCells = %d\n\n", t.goName, t.numCells)
	if t.sampleRate != 0 {
		fmt.Fprintf(&b, "// %sSampleRate is the sample rate (Hz) %s was originally recorded/generated at,\n// i.e. the Oscil update rate that reproduces it at its \"natural\" pitch.\n", t.goName, t.goName)
		fmt.Fprintf(&b, "const %sSampleRate = %d\n\n", t.goName, t.sampleRate)
	}
	fmt.Fprintf(&b, "//go:embed tablesdata/%s.bin\n", t.stem)
	fmt.Fprintf(&b, "var %sBytes []byte\n\n", t.goName)
	fmt.Fprintf(&b, "// %s is a %s-cell []%s wavetable, ported from Mozzi/tables/%s.h.\n", t.goName, fmt.Sprint(t.numCells), goType, t.stem)
	fmt.Fprintf(&b, "// The backing array lives in flash (via go:embed); reading it costs no RAM.\n")
	fmt.Fprintf(&b, "var %s = %s(%sBytes)\n", t.goName, convFunc, t.goName)

	check(os.WriteFile(path, []byte(b.String()), 0o644))
}

func writeDoc(path string, docLines []string) {
	var b strings.Builder
	b.WriteString("// Code generated by tools/gentables; DO NOT EDIT.\n\n")
	b.WriteString("// Package tables holds the wavetables ported from Mozzi/tables/*.h,\n")
	b.WriteString("// for use with mozzi/oscil.Oscil. Each table is a flash-resident slice\n")
	b.WriteString("// (see conv.go) with no RAM cost until you actually read from it.\n")
	b.WriteString("//\n")
	b.WriteString("// Tables (name, cell count, and, where meaningful, the update rate that\n")
	b.WriteString("// plays it back at its original pitch):\n")
	for _, l := range docLines {
		b.WriteString(l)
		b.WriteString("\n")
	}
	b.WriteString("//\n")
	b.WriteString("// The BandLimited_SAW/SQUARE/TRI table sets (multi-resolution, anti-aliased\n")
	b.WriteString("// wavetables for MetaOscil-style playback) were intentionally left out to\n")
	b.WriteString("// keep the default flash footprint small; regenerate with\n")
	b.WriteString("// `go run ./tools/gentables <path> tables/bandlimited` if you need them.\n")
	b.WriteString("package tables\n")
	check(os.WriteFile(path, []byte(b.String()), 0o644))
}

func check(err error) {
	if err != nil {
		fatalf("%v", err)
	}
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
