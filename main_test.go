package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, dir string, name string, content string) string {
	t.Helper()
	file := filepath.Join(dir, name)
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	return file
}

func equalGenerators(a [][]float64, b [][]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}

func TestParseComplexFiltrationFileValidGroupedFormat(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "complex.csv", "# header\n4,2\n0,1,\"[[1,0],[0,1]]\"\n1,2,\"[[2,1]]\"\n")

	filtration, err := parseComplexFiltrationFile(file)
	if err != nil {
		t.Fatalf("parseComplexFiltrationFile failed: %v", err)
	}
	if filtration.numVertices != 4 {
		t.Fatalf("numVertices: got %d, want 4", filtration.numVertices)
	}
	if filtration.paramDim != 2 {
		t.Fatalf("paramDim: got %d, want 2", filtration.paramDim)
	}
	if len(filtration.oneSimplexGenerators) != 2 {
		t.Fatalf("oneSimplexGenerators: got %d, want 2", len(filtration.oneSimplexGenerators))
	}

	got := filtration.oneSimplexGenerators[oneSimplexKey{i: 0, j: 1}]
	want := [][]float64{{0, 1}, {1, 0}}
	if !equalGenerators(got, want) {
		t.Fatalf("unexpected generators for (0,1): got %v want %v", got, want)
	}
}

func TestParseComplexFiltrationFilePrunesDominatedGenerators(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "complex.csv", "3,2\n0,1,\"[[1,3],[2,3],[5,5]]\"\n")

	filtration, err := parseComplexFiltrationFile(file)
	if err != nil {
		t.Fatalf("parseComplexFiltrationFile failed: %v", err)
	}

	got := filtration.oneSimplexGenerators[oneSimplexKey{i: 0, j: 1}]
	want := [][]float64{{1, 3}}
	if !equalGenerators(got, want) {
		t.Fatalf("unexpected normalized generators: got %v want %v", got, want)
	}
}

func TestParseComplexFiltrationFileRejectsMalformedGeneratorJSON(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "complex.csv", "3,2\n0,1,\"[(1,2)]\"\n")

	_, err := parseComplexFiltrationFile(file)
	if err == nil {
		t.Fatal("expected parseComplexFiltrationFile to fail on malformed generator JSON")
	}
	if !strings.Contains(err.Error(), "invalid generator list") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseComplexFiltrationFileRejectsDuplicateEdgeRows(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "complex.csv", "3,2\n0,1,\"[[1,0]]\"\n0,1,\"[[0,1]]\"\n")

	_, err := parseComplexFiltrationFile(file)
	if err == nil {
		t.Fatal("expected duplicate-edge error")
	}
	if !strings.Contains(err.Error(), "duplicate 1-simplex") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseComplexFiltrationFileRejectsComparableLengthMismatch(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "complex.csv", "3,2\n0,1,\"[[1]]\"\n")

	_, err := parseComplexFiltrationFile(file)
	if err == nil {
		t.Fatal("expected generator-length validation error")
	}
	if !strings.Contains(err.Error(), "has length") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePathInlineAndValidate(t *testing.T) {
	path, err := parsePathInline("[[0,0],[1,0],[1,2]]")
	if err != nil {
		t.Fatalf("parsePathInline failed: %v", err)
	}
	if err := validatePath(path, 2); err != nil {
		t.Fatalf("validatePath failed: %v", err)
	}
}

func TestValidatePathRejectsNonMonotone(t *testing.T) {
	path, err := parsePathInline("[[0,1],[1,0]]")
	if err != nil {
		t.Fatalf("parsePathInline failed: %v", err)
	}
	if err := validatePath(path, 2); err == nil {
		t.Fatal("expected validatePath to fail for non-monotone path")
	}
}

func TestParsePathFileMultipleLinesAndComments(t *testing.T) {
	dir := t.TempDir()
	file := writeTestFile(t, dir, "paths.txt", "# comment\n[[0],[1]]\n\n[[1],[2]]\n")

	paths, err := parsePathFile(file)
	if err != nil {
		t.Fatalf("parsePathFile failed: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("paths: got %d, want 2", len(paths))
	}
}

func TestEntryIndexAntichainTakesEarliestGenerator(t *testing.T) {
	path := [][]float64{{0, 0}, {1, 0}, {1, 1}, {2, 1}}
	generators := [][]float64{{2, 0}, {0, 1}}
	got := entryIndexAntichain(generators, path, len(path)+1)
	// [0,1] enters at step 3; [2,0] enters at step 4 => use earliest (3)
	if got != 3 {
		t.Fatalf("entryIndexAntichain: got %d, want 3", got)
	}
}

func TestMissingEdgeNeverAppearsInRow(t *testing.T) {
	filtration := complexFiltrationData{
		numVertices: 3,
		paramDim:    2,
		oneSimplexGenerators: map[oneSimplexKey][][]float64{
			{i: 0, j: 1}: {{1, 0}},
			{i: 1, j: 2}: {{2, 0}},
		},
	}
	path := [][]float64{{1, 0}, {2, 0}}

	row := rowForVertex(2, filtration, path)
	if row != "3,2" {
		t.Fatalf("rowForVertex(2): got %q, want %q", row, "3,2")
	}
}

func TestWriteAuxMatrixDeterministicOrder(t *testing.T) {
	filtration := complexFiltrationData{
		numVertices: 4,
		paramDim:    1,
		oneSimplexGenerators: map[oneSimplexKey][][]float64{
			{i: 0, j: 1}: {{1}},
			{i: 0, j: 2}: {{2}},
			{i: 1, j: 2}: {{1}},
			{i: 0, j: 3}: {{3}},
			{i: 1, j: 3}: {{2}},
			{i: 2, j: 3}: {{1}},
		},
	}
	path := [][]float64{{1}, {2}, {3}}

	var sb strings.Builder
	w := bufio.NewWriter(&sb)
	if err := writeAuxMatrix(w, filtration, path, 2); err != nil {
		t.Fatalf("writeAuxMatrix failed: %v", err)
	}

	got := strings.TrimSpace(sb.String())
	want := strings.Join([]string{"1", "2,1", "3,2,1"}, "\n")
	if got != want {
		t.Fatalf("matrix output mismatch:\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestTranslateRipserOutputHandlesInfinity(t *testing.T) {
	raw := strings.Join([]string{
		"persistent homology intervals in dim 1:",
		" [1, )",
	}, "\n")
	path := [][]float64{{0, 0}, {1, 1}}

	translated, err := translateRipserOutput(raw, path)
	if err != nil {
		t.Fatalf("translateRipserOutput failed: %v", err)
	}
	if !strings.Contains(translated, "[[0,0], inf):") {
		t.Fatalf("unexpected translation output: %s", translated)
	}
}

func TestNormalizeArgsForRipserFlagBare(t *testing.T) {
	in := []string{"murit", "--complex", "c.csv", "--path", "p.paths", "--ripser", "--dim", "1"}
	got := normalizeArgsForRipserFlag(in)
	want := []string{"murit", "--complex", "c.csv", "--path", "p.paths", "--ripser=", "--dim", "1"}

	if len(got) != len(want) {
		t.Fatalf("arg length mismatch: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg mismatch at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeArgsForRipserFlagWithValue(t *testing.T) {
	in := []string{"murit", "--complex", "c.csv", "--path", "p.paths", "--ripser", "/tmp/ripser", "--dim", "1"}
	got := normalizeArgsForRipserFlag(in)
	want := []string{"murit", "--complex", "c.csv", "--path", "p.paths", "--ripser", "/tmp/ripser", "--dim", "1"}

	if len(got) != len(want) {
		t.Fatalf("arg length mismatch: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg mismatch at %d: got %q want %q", i, got[i], want[i])
		}
	}
}

func TestResolveRipserExecutableRejectsMissingPath(t *testing.T) {
	_, err := resolveRipserExecutable("/definitely/not/here/murit-ripser")
	if err == nil {
		t.Fatal("expected resolveRipserExecutable to fail for missing path")
	}
	if !strings.Contains(err.Error(), "failed to locate ripser executable") {
		t.Fatalf("unexpected error: %v", err)
	}
}
