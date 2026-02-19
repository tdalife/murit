/*
MuRiT: Efficient Computation of Pathwise Persistence Barcodes in Multi-Filtered Flag Complexes via Vietoris-Rips Transformations
https://doi.org/10.48550/arXiv.2207.03394

MIT License
*/

package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type oneSimplexKey struct {
	i int
	j int
}

type complexFiltrationData struct {
	numVertices          int
	paramDim             int
	oneSimplexGenerators map[oneSimplexKey][][]float64
}

type rowResult struct {
	row int
	out string
	err error
}

const ripserFlagDisabled = "__murit_ripser_disabled__"

func leq(a []float64, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] > b[i] {
			return false
		}
	}
	return true
}

func equalVector(a []float64, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func normalizeGenerators(generators [][]float64) [][]float64 {
	unique := make([][]float64, 0, len(generators))
	for _, g := range generators {
		alreadySeen := false
		for _, u := range unique {
			if equalVector(g, u) {
				alreadySeen = true
				break
			}
		}
		if !alreadySeen {
			copyVec := append([]float64(nil), g...)
			unique = append(unique, copyVec)
		}
	}

	antichain := make([][]float64, 0, len(unique))
	for i, g := range unique {
		dominated := false
		for j, h := range unique {
			if i == j {
				continue
			}
			if leq(h, g) {
				dominated = true
				break
			}
		}
		if !dominated {
			antichain = append(antichain, g)
		}
	}

	// Canonical order for deterministic output/tests: lexicographic.
	for i := 0; i < len(antichain); i++ {
		for j := i + 1; j < len(antichain); j++ {
			swap := false
			for c := 0; c < len(antichain[i]) && c < len(antichain[j]); c++ {
				if antichain[j][c] < antichain[i][c] {
					swap = true
					break
				}
				if antichain[j][c] > antichain[i][c] {
					break
				}
			}
			if swap {
				antichain[i], antichain[j] = antichain[j], antichain[i]
			}
		}
	}

	return antichain
}

func parseCSVLine(line string) ([]string, error) {
	reader := csv.NewReader(strings.NewReader(line))
	reader.TrimLeadingSpace = true
	fields, err := reader.Read()
	if err != nil {
		return nil, err
	}
	for i := range fields {
		fields[i] = strings.TrimSpace(fields[i])
	}
	return fields, nil
}

func parseComplexFiltrationFile(fileName string) (complexFiltrationData, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return complexFiltrationData{}, fmt.Errorf("failed to open complex filtration file '%s': %w", fileName, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, bufio.MaxScanTokenSize*64)

	lineNo := 0
	headerSeen := false
	filtration := complexFiltrationData{}

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields, err := parseCSVLine(line)
		if err != nil {
			return complexFiltrationData{}, fmt.Errorf("invalid CSV row at line %d: %w", lineNo, err)
		}
		if !headerSeen {
			if len(fields) != 2 {
				return complexFiltrationData{}, fmt.Errorf("complex header must be 'N,k' at line %d", lineNo)
			}
			n, err := strconv.Atoi(fields[0])
			if err != nil {
				return complexFiltrationData{}, fmt.Errorf("invalid N in complex header at line %d: %w", lineNo, err)
			}
			k, err := strconv.Atoi(fields[1])
			if err != nil {
				return complexFiltrationData{}, fmt.Errorf("invalid k in complex header at line %d: %w", lineNo, err)
			}
			if n < 1 {
				return complexFiltrationData{}, fmt.Errorf("invalid complex header at line %d: N must be >= 1", lineNo)
			}
			if k < 1 {
				return complexFiltrationData{}, fmt.Errorf("invalid complex header at line %d: k must be >= 1", lineNo)
			}
			filtration = complexFiltrationData{
				numVertices:          n,
				paramDim:             k,
				oneSimplexGenerators: make(map[oneSimplexKey][][]float64),
			}
			headerSeen = true
			continue
		}

		if len(fields) != 3 {
			return complexFiltrationData{}, fmt.Errorf("invalid 1-simplex row at line %d: expected 3 columns 'i,j,\"[[...]]\"', got %d", lineNo, len(fields))
		}

		i, err := strconv.Atoi(fields[0])
		if err != nil {
			return complexFiltrationData{}, fmt.Errorf("invalid 1-simplex row at line %d: bad i index: %w", lineNo, err)
		}
		j, err := strconv.Atoi(fields[1])
		if err != nil {
			return complexFiltrationData{}, fmt.Errorf("invalid 1-simplex row at line %d: bad j index: %w", lineNo, err)
		}
		if i < 0 || j < 0 || i >= filtration.numVertices || j >= filtration.numVertices {
			return complexFiltrationData{}, fmt.Errorf("invalid 1-simplex row at line %d: indices must satisfy 0 <= i,j < N", lineNo)
		}
		if i >= j {
			return complexFiltrationData{}, fmt.Errorf("invalid 1-simplex row at line %d: require i < j", lineNo)
		}

		key := oneSimplexKey{i: i, j: j}
		if _, exists := filtration.oneSimplexGenerators[key]; exists {
			return complexFiltrationData{}, fmt.Errorf("duplicate 1-simplex (%d,%d) at line %d", i, j, lineNo)
		}

		var generators [][]float64
		if err := json.Unmarshal([]byte(fields[2]), &generators); err != nil {
			return complexFiltrationData{}, fmt.Errorf("invalid generator list at line %d: %w", lineNo, err)
		}
		if len(generators) == 0 {
			return complexFiltrationData{}, fmt.Errorf("invalid generator list at line %d: list must be non-empty", lineNo)
		}
		for gIdx, g := range generators {
			if len(g) != filtration.paramDim {
				return complexFiltrationData{}, fmt.Errorf(
					"invalid generator list at line %d: generator %d has length %d, expected %d",
					lineNo, gIdx+1, len(g), filtration.paramDim,
				)
			}
		}
		filtration.oneSimplexGenerators[key] = normalizeGenerators(generators)
	}

	if err := scanner.Err(); err != nil {
		return complexFiltrationData{}, fmt.Errorf("error while reading complex filtration file '%s': %w", fileName, err)
	}
	if !headerSeen {
		return complexFiltrationData{}, errors.New("complex filtration file is empty (no 'N,k' header found)")
	}

	return filtration, nil
}

func parsePathInline(input string) ([][]float64, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, errors.New("path is empty")
	}

	var path [][]float64
	if err := json.Unmarshal([]byte(trimmed), &path); err != nil {
		return nil, fmt.Errorf("invalid path literal, expected JSON array of vectors, e.g. [[0,0],[1,2]]: %w", err)
	}
	if len(path) == 0 {
		return nil, errors.New("path must contain at least one vector")
	}
	return path, nil
}

func parsePathFile(fileName string) ([][][]float64, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open path file '%s': %w", fileName, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, bufio.MaxScanTokenSize*64)

	lineNo := 0
	paths := make([][][]float64, 0)
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		path, err := parsePathInline(line)
		if err != nil {
			return nil, fmt.Errorf("invalid path at '%s:%d': %w", fileName, lineNo, err)
		}
		paths = append(paths, path)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error while reading path file '%s': %w", fileName, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("path file '%s' does not contain any valid path lines", fileName)
	}
	return paths, nil
}

func validatePath(path [][]float64, k int) error {
	if len(path) == 0 {
		return errors.New("path cannot be empty")
	}
	for idx, point := range path {
		if len(point) != k {
			return fmt.Errorf("invalid path vector length at step %d: expected %d, got %d", idx+1, k, len(point))
		}
	}
	for i := 1; i < len(path); i++ {
		if !leq(path[i-1], path[i]) {
			return fmt.Errorf("path is not totally ordered at step %d: %v !<= %v", i, path[i-1], path[i])
		}
	}
	return nil
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func formatVector(values []float64) string {
	parts := make([]string, len(values))
	for i := range values {
		parts[i] = formatFloat(values[i])
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func formatPath(path [][]float64) string {
	parts := make([]string, len(path))
	for i := range path {
		parts[i] = formatVector(path[i])
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func entryIndexAntichain(generators [][]float64, path [][]float64, neverIndex int) int {
	best := neverIndex
	for _, g := range generators {
		for idx, point := range path {
			if leq(g, point) {
				entry := idx + 1
				if entry < best {
					best = entry
				}
				break
			}
		}
	}
	return best
}

func rowForVertex(i int, filtration complexFiltrationData, path [][]float64) string {
	if i == 0 {
		return ""
	}
	neverIndex := len(path) + 1
	var sb strings.Builder
	for j := 0; j < i; j++ {
		value := neverIndex
		if generators, ok := filtration.oneSimplexGenerators[oneSimplexKey{i: j, j: i}]; ok {
			value = entryIndexAntichain(generators, path, neverIndex)
		}
		sb.WriteString(strconv.Itoa(value))
		if j+1 < i {
			sb.WriteByte(',')
		}
	}
	return sb.String()
}

func writeAuxMatrix(outWriter *bufio.Writer, filtration complexFiltrationData, path [][]float64, numThreads int) error {
	if numThreads < 1 {
		numThreads = 1
	}
	if filtration.numVertices <= 1 {
		return outWriter.Flush()
	}

	jobs := make(chan int, numThreads*2)
	results := make(chan rowResult, numThreads*2)

	var wg sync.WaitGroup
	for workerID := 0; workerID < numThreads; workerID++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for row := range jobs {
				results <- rowResult{row: row, out: rowForVertex(row, filtration, path)}
			}
		}()
	}

	go func() {
		for row := 1; row < filtration.numVertices; row++ {
			jobs <- row
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	nextRow := 1
	pending := make(map[int]string)
	var firstErr error

	for result := range results {
		if result.err != nil && firstErr == nil {
			firstErr = result.err
		}
		pending[result.row] = result.out

		for {
			line, ok := pending[nextRow]
			if !ok {
				break
			}
			if _, err := outWriter.WriteString(line + "\n"); err != nil && firstErr == nil {
				firstErr = err
			}
			delete(pending, nextRow)
			nextRow++
		}
	}

	if firstErr != nil {
		return firstErr
	}

	if err := outWriter.Flush(); err != nil {
		return err
	}
	return nil
}

func sanitizeRipserLine(line string) string {
	line = strings.ReplaceAll(line, "\r", "")
	var sb strings.Builder
	for i := 0; i < len(line); i++ {
		if line[i] == 0x1b {
			// Strip CSI ANSI escape sequences such as ESC[K or ESC[31m.
			if i+1 < len(line) && line[i+1] == '[' {
				i += 2
				for i < len(line) {
					c := line[i]
					if c >= '@' && c <= '~' {
						break
					}
					i++
				}
				continue
			}
			continue
		}
		sb.WriteByte(line[i])
	}
	return strings.TrimRight(sb.String(), " \t")
}

func isInfinityToken(token string) bool {
	normalized := strings.ToLower(strings.TrimSpace(strings.Trim(token, ":")))
	return normalized == "" || normalized == "inf" || normalized == "+inf" || normalized == "infinity" || normalized == "+infinity"
}

func parseIntervalLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "[") {
		return "", "", false
	}
	closePos := strings.Index(trimmed, ")")
	if closePos <= 0 {
		return "", "", false
	}
	inside := strings.TrimSpace(trimmed[1:closePos])
	parts := strings.SplitN(inside, ",", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	birth := strings.TrimSpace(parts[0])
	death := strings.TrimSpace(parts[1])
	return birth, death, true
}

func vectorAt(path [][]float64, oneBasedIndex int) ([]float64, bool) {
	if oneBasedIndex < 1 || oneBasedIndex > len(path) {
		return nil, false
	}
	return path[oneBasedIndex-1], true
}

func translateRipserOutput(ripserOutput string, path [][]float64) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(ripserOutput))
	buffer := make([]byte, 64*1024)
	scanner.Buffer(buffer, bufio.MaxScanTokenSize*64)

	var sb strings.Builder
	printFlag := true
	for scanner.Scan() {
		line := sanitizeRipserLine(scanner.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.Contains(line, "persistent homology") && strings.Contains(line, "dim 0") {
			printFlag = false
		} else if strings.Contains(line, "persistent homology") {
			printFlag = true
		}

		if !printFlag {
			continue
		}

		birthToken, deathToken, ok := parseIntervalLine(line)
		if ok {
			birthIndex, birthErr := strconv.Atoi(strings.TrimSpace(strings.Trim(birthToken, ":")))
			if birthErr == nil {
				birthPoint, birthOK := vectorAt(path, birthIndex)
				if birthOK {
					deathText := "inf"
					if !isInfinityToken(deathToken) {
						deathIndex, deathErr := strconv.Atoi(strings.TrimSpace(strings.Trim(deathToken, ":")))
						if deathErr == nil {
							if deathPoint, deathOK := vectorAt(path, deathIndex); deathOK {
								deathText = formatVector(deathPoint)
							}
						}
					}
					sb.WriteString(" [")
					sb.WriteString(formatVector(birthPoint))
					sb.WriteString(", ")
					sb.WriteString(deathText)
					sb.WriteString("):\n")
					continue
				}
			}
		}

		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func runRipserAndTranslate(ripserExecutable string, auxFileName string, path [][]float64, dim string, modulus string, ratio string) (string, error) {
	ripserArgs := []string{"--format", "lower-distance"}
	if dim != "" {
		ripserArgs = append(ripserArgs, "--dim", dim)
	}
	ripserArgs = append(ripserArgs, "--threshold", strconv.Itoa(len(path)))
	if modulus != "" {
		ripserArgs = append(ripserArgs, "--modulus", modulus)
	}
	if ratio != "" {
		ripserArgs = append(ripserArgs, "--ratio", ratio)
	}
	ripserArgs = append(ripserArgs, auxFileName)

	ripserCmd := exec.Command(ripserExecutable, ripserArgs...)
	output, err := ripserCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run ripser (%s): %w\n%s", ripserExecutable, err, string(output))
	}

	translated, err := translateRipserOutput(string(output), path)
	if err != nil {
		return "", fmt.Errorf("failed to parse ripser output: %w", err)
	}
	return translated, nil
}

func inputOutputBase(inputFileName string) string {
	dir := filepath.Dir(inputFileName)
	base := filepath.Base(inputFileName)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, stem)
}

func writeAuxToFile(fileName string, filtration complexFiltrationData, path [][]float64, numThreads int) error {
	outFile, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create auxiliary file '%s': %w", fileName, err)
	}
	defer outFile.Close()

	outWriter := bufio.NewWriter(outFile)
	if err := writeAuxMatrix(outWriter, filtration, path, numThreads); err != nil {
		return fmt.Errorf("failed to write auxiliary matrix '%s': %w", fileName, err)
	}
	return nil
}

func parseThreads(raw string) (int, error) {
	if raw == "" {
		return runtime.NumCPU(), nil
	}
	threads, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid --threads value '%s': %w", raw, err)
	}
	if threads < 1 {
		return 0, errors.New("--threads must be >= 1")
	}
	return threads, nil
}

func normalizeArgsForRipserFlag(args []string) []string {
	if len(args) == 0 {
		return args
	}
	normalized := make([]string, 0, len(args))
	normalized = append(normalized, args[0])

	for idx := 1; idx < len(args); idx++ {
		token := args[idx]
		if token == "--ripser" || token == "-ripser" {
			if idx+1 >= len(args) || strings.HasPrefix(args[idx+1], "-") {
				normalized = append(normalized, token+"=")
			} else {
				normalized = append(normalized, token, args[idx+1])
				idx++
			}
			continue
		}
		normalized = append(normalized, token)
	}

	return normalized
}

func resolveRipserExecutable(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	candidate := trimmed
	if candidate == "" {
		candidate = "ripser"
	}

	resolved, err := exec.LookPath(candidate)
	if err != nil {
		if trimmed == "" {
			return "", errors.New("failed to locate ripser in PATH; pass --ripser <path-to-ripser> or install ripser")
		}
		return "", fmt.Errorf("failed to locate ripser executable '%s': %w", candidate, err)
	}
	return resolved, nil
}

func main() {
	var complexFileName string
	var pathInput string
	var threadsRaw string

	var verbose bool
	var help bool

	var ripserInput string
	var ripserDim string
	var ripserModulus string
	var ripserRatio string

	flag.StringVar(&complexFileName, "complex", "", "file name of sparse multifiltration for a clique complex (CSV): header 'N,k', rows 'i,j,\"[[v1...],[v2...]]\"'.")
	flag.StringVar(&pathInput, "path", "", "path literal (JSON array of vectors) OR path file name (one path literal per non-empty line).")

	flag.StringVar(&threadsRaw, "threads", "", "number of threads (default: runtime.NumCPU())")
	flag.BoolVar(&verbose, "verbose", false, "Show status messages (default: false)")
	flag.BoolVar(&help, "help", false, "Show this help message")
	flag.StringVar(&ripserInput, "ripser", ripserFlagDisabled, "run ripser on auxiliary entry-index matrix; optionally pass executable path.\n  Use '--ripser' for PATH lookup or '--ripser /path/to/ripser'.")
	flag.StringVar(&ripserDim, "dim", "1", "compute persistent homology up to dimension k (default: 1).")
	flag.StringVar(&ripserModulus, "modulus", "", "compute homology with coefficients in the prime field Z/pZ (default: 2).")
	flag.StringVar(&ripserRatio, "ratio", "", "only show persistence pairs with death/birth ratio > r")

	flag.Usage = func() {
		fmt.Printf("Usage:\n")
		fmt.Printf("murit --complex <filename> --path <inline-path-or-path-file> [--options]\n\n")
		fmt.Printf("Input formats:\n")
		fmt.Printf("  multifiltration file for a clique complex:\n")
		fmt.Printf("    # comments allowed\n")
		fmt.Printf("    N,k\n")
		fmt.Printf("    i,j,\"[[a1,...,ak],[b1,...,bk],...]\"\n")
		fmt.Printf("    (one row per 1-simplex/edge; generator list auto-normalized to minimal antichain)\n\n")
		fmt.Printf("  path literal:\n")
		fmt.Printf("    JSON array of vectors.\n")
		fmt.Printf("    [[a1,a2,...],[b1,b2,...],...]\n")
		fmt.Printf("  path file:\n")
		fmt.Printf("    one path literal per non-empty line, using the exact same format as above.\n")
		fmt.Printf("    comments with '#'.\n\n")
		fmt.Printf("Command Line Arguments\n")
		arguments := []string{"complex", "path", "threads", "verbose", "help", "ripser", "dim", "modulus", "ratio"}
		for _, name := range arguments {
			option := flag.CommandLine.Lookup(name)
			fmt.Printf("-%s\n", option.Name)
			fmt.Printf("  %s\n", option.Usage)
		}
	}

	os.Args = normalizeArgsForRipserFlag(os.Args)
	flag.Parse()

	if help {
		flag.Usage()
		return
	}

	if complexFileName == "" {
		log.Fatal("complex filtration file required (--complex)")
	}
	if pathInput == "" {
		log.Fatal("path input required (--path)")
	}

	threads, err := parseThreads(threadsRaw)
	if err != nil {
		log.Fatalf("thread parsing error: %v", err)
	}

	runRipser := ripserInput != ripserFlagDisabled
	ripserExecutable := ""
	if runRipser {
		ripserExecutable, err = resolveRipserExecutable(ripserInput)
		if err != nil {
			log.Fatal(err)
		}
	}

	filtration, err := parseComplexFiltrationFile(complexFileName)
	if err != nil {
		log.Fatal(err)
	}

	pathInputTrimmed := strings.TrimSpace(pathInput)
	inlinePathMode := strings.HasPrefix(pathInputTrimmed, "[")

	var paths [][][]float64
	if inlinePathMode {
		path, err := parsePathInline(pathInputTrimmed)
		if err != nil {
			log.Fatal(err)
		}
		paths = [][][]float64{path}
	} else {
		paths, err = parsePathFile(pathInputTrimmed)
		if err != nil {
			log.Fatal(err)
		}
	}

	for pathIdx, path := range paths {
		if err := validatePath(path, filtration.paramDim); err != nil {
			if inlinePathMode {
				log.Fatalf("invalid path: %v", err)
			}
			log.Fatalf("invalid path at line %d of '%s': %v", pathIdx+1, pathInputTrimmed, err)
		}
	}

	if verbose {
		fmt.Println("Clique Complex Filtration")
		fmt.Printf("N=%d vertices, k=%d parameters, listed 1-simplices=%d\n", filtration.numVertices, filtration.paramDim, len(filtration.oneSimplexGenerators))
		fmt.Printf("Paths=%d\n", len(paths))
	}

	if inlinePathMode {
		path := paths[0]
		if verbose {
			fmt.Println("\nPath")
			fmt.Println(formatPath(path))
		}

		if !runRipser {
			if verbose {
				fmt.Println("\nAuxiliary Entry-Index Matrix")
			}
			outWriter := bufio.NewWriter(os.Stdout)
			if err := writeAuxMatrix(outWriter, filtration, path, threads); err != nil {
				log.Fatalf("failed to create auxiliary entry-index matrix: %v", err)
			}
			return
		}

		tempFile, err := os.CreateTemp(filepath.Dir(complexFileName), "murit-inline-*.aux")
		if err != nil {
			log.Fatalf("failed to create temporary auxiliary file: %v", err)
		}
		tempName := tempFile.Name()
		tempFile.Close()
		defer os.Remove(tempName)

		if err := writeAuxToFile(tempName, filtration, path, threads); err != nil {
			log.Fatal(err)
		}

		if verbose {
			fmt.Println("\nRipser")
		}
		translated, err := runRipserAndTranslate(ripserExecutable, tempName, path, ripserDim, ripserModulus, ripserRatio)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(translated)
		return
	}

	base := inputOutputBase(complexFileName)
	type pathSummary struct {
		pathIndex   int
		auxFile     string
		ripserFile  string
		pathLiteral string
	}
	summaries := make([]pathSummary, 0, len(paths))

	for idx, path := range paths {
		pathNumber := idx + 1
		auxFile := fmt.Sprintf("%s_path%02d.aux", base, pathNumber)

		if verbose {
			fmt.Printf("\nPath %d\n", pathNumber)
			fmt.Println(formatPath(path))
			fmt.Printf("Writing auxiliary matrix to %s\n", auxFile)
		}

		if err := writeAuxToFile(auxFile, filtration, path, threads); err != nil {
			log.Fatal(err)
		}

		summary := pathSummary{
			pathIndex:   pathNumber,
			auxFile:     auxFile,
			pathLiteral: formatPath(path),
		}

		if runRipser {
			if verbose {
				fmt.Printf("Running ripser for path %d\n", pathNumber)
			}

			translated, err := runRipserAndTranslate(ripserExecutable, auxFile, path, ripserDim, ripserModulus, ripserRatio)
			if err != nil {
				log.Fatalf("ripser failed for path %d: %v", pathNumber, err)
			}

			ripserFile := fmt.Sprintf("%s_path%02d.ripser", base, pathNumber)
			if err := os.WriteFile(ripserFile, []byte(translated), 0o644); err != nil {
				log.Fatalf("failed to write ripser output file '%s': %v", ripserFile, err)
			}
			summary.ripserFile = ripserFile
		}

		summaries = append(summaries, summary)
	}

	fmt.Printf("Processed %d path(s) from %s\n", len(summaries), pathInputTrimmed)
	for _, summary := range summaries {
		if runRipser {
			fmt.Printf("Path %02d: aux=%s, ripser=%s\n", summary.pathIndex, summary.auxFile, summary.ripserFile)
		} else {
			fmt.Printf("Path %02d: aux=%s\n", summary.pathIndex, summary.auxFile)
		}
	}
}
