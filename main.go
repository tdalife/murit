/*
MuRiT: Efficient Computation of Pathwise Persistence Barcodes in Multi-Filtered Flag Complexes via Vietoris-Rips Transformations
https://doi.org/10.48550/arXiv.2207.03394

MIT License
*/

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"bufio"
	"strconv"
	"runtime"
	"math"
	"os/exec"
	"strings"
  "path/filepath"
  "crypto/sha256"
  "time"
)


// define two kinds of structs for handling input of parallelized workers

type workload struct {
  i    int
  text string
}

type args struct{
  path [][]float64
  minima_list [][][]float64
}


// implementation of standard partial order on R^n
func leq(poset_element1 []float64, poset_element2 []float64) bool {
	var L int
	if len(poset_element1) <= len(poset_element2) {
		L = len(poset_element1)
	} else {
		L = len(poset_element2)
	}
	for i:= 0; i<L; i++ {
		if (poset_element1[i] > poset_element2[i]) {
			return false
		}
	}
	return true
}

// implementation of equality check on R^n
func equal(poset_element1 []float64, poset_element2 []float64) bool {
	var L int
	if len(poset_element1) <= len(poset_element2) {
		L = len(poset_element1)
	} else {
		L = len(poset_element2)
	}
	for i:= 0; i<L; i++ {
		if (poset_element1[i] != poset_element2[i]) {
			return false
		}
	}
	return true
}


// find index for which a data point first enters a totally ordered subfiltration from list of filtration minima
func get_index_of_entry(minima [][]float64, path [][]float64) int{
	// for each filtration step along the subfiltration, check if one of the minima of the data point lies below.
	// If so, we found the point of entry
	for k, x := range path {
		for _, minimum := range minima{
			if leq(minimum, x){
				return k+1 // shift from zero- to one-indexing!
			}
		}
	// if the point is not contained in the path at all, set index to (maximal filtration value + 1) <- equivalent to infty
	}
	return len(path) // note that this already contains the shift from zero- to one-indexing!
}


//
// Reader function
func reader(in *os.File, out []chan workload) {
  var in_scanner *bufio.Scanner
  in_scanner = bufio.NewScanner(in)

  i := 1
  channel_i := 0
  // Increase buffer size, MaxScanTokenSize is too low!
  // See: https://pkg.go.dev/bufio?utm_source=gopls#Scanner.Buffer
  buffer := make([]byte, 64000)
  in_scanner.Buffer(buffer, math.MaxInt)

  for in_scanner.Scan() {
    out[channel_i] <- workload{
      i:    i,
      text: in_scanner.Text(),
    }
    i++

    // channel_i = (channel_i + 1) modulo number of channels
    channel_i++
    if channel_i == len(out) {
      channel_i = 0
    }
  }

  // close Channel
  for _, o := range out {
    close(o)
  }
}


// Worker function
func worker(in chan workload, out chan string, b args) {
  var sb strings.Builder

  for w := range in {
    // clear string builder for new matrix line
    sb.Reset()

    // Split line of distance matrix into the single distances at separator
    splitLine := strings.Split(w.text, ",")

    for j, token := range splitLine {
      // convert distance string to int
      distance, err := strconv.ParseFloat(token, 64)
      if err != nil {
        log.Fatalf("Distance conversion error: %v", err)
      }
			// Append distance of x_i and x_j to the start of each filtration value of the given minimum
			var minima_i [][]float64
			for _, minimum := range b.minima_list[w.i]{
				minima_i = append(minima_i, append([]float64{distance}, minimum...))
			}
			var minima_j [][]float64
			for _, minimum := range b.minima_list[j]{
				minima_j = append(minima_j, append([]float64{distance}, minimum...))
			}
      // Determine modified distance for pair of datapoints (x_i,x_j).
      // modified distance is the index of entry for the edge (x_i,x_j)
			// the edge is present as soon as both x_i and x_j have entered
			var modified_distance int
			index_of_entry_i := get_index_of_entry(minima_i, b.path)
			index_of_entry_j := get_index_of_entry(minima_j, b.path)
			// use maximum of the two
			if index_of_entry_i >= index_of_entry_j {
				modified_distance = index_of_entry_i
			} else {
				modified_distance = index_of_entry_j
			}
			// fmt.Println(minima_i, index_of_entry_i, "-", minima_j, index_of_entry_j, "-", modified_distance)
      // Concatenate modified distance to current matrix line
      sb.WriteString(strconv.Itoa(modified_distance)) // write modified distance
      sb.WriteByte(',')                               // write separator
    }
    // Send modified matrix line to out channel (to be used by writer)
    joinLine := sb.String()
    out <- joinLine[:len(joinLine)-1]
  }

  // Close Channel
  close(out)
}


// Writer function
func writer(in []chan string, out_writer *bufio.Writer) {

  for {
    for i := 0; i < len(in); i++ {
      // Read from channel
      line, ok := <-in[i]

      // finish, when channel is closed
      if !ok {
        err := out_writer.Flush()
        if err != nil {
          log.Fatalf("Failed to flush to out_writer: %v", err)
        }
        return
      }

      // Write line to pipe
      _, err := out_writer.WriteString(line)
      if err != nil {
        log.Fatalf("Failed to write to out_writer: %v", err)
      }
      // append linebreak to line
      _, err = out_writer.WriteString("\n")
      if err != nil {
        log.Fatalf("Failed to write to out_writer: %v", err)
      }
    }
  }
}

// hash function for generating filename of auxiliary file (mainly for future development)
func hash(s string) string {
        h := sha256.New()
        h.Write([]byte(s))
        return fmt.Sprintf("%x", h.Sum(nil))
}








//-------------------------------------------------------------------------
//----------------         main        ------------------------------------
//-------------------------------------------------------------------------
func main() {
	var dist_file_name string
	var minima_file_name string
	var path_input string
	var threads string

	var verbose bool
	var help bool

	var ripser bool
	var ripser_dim string
	var ripser_threshold string
	var ripser_modulus string
	var ripser_ratio string

	var aux_file_name string
	var aux_description string


	//
	// Command Line Options
	//

	flag.StringVar(&dist_file_name, "dist", "", "file name of lower-triangular distance matrix.")

	aux_description=`file name of pointwise minima annotation.

  file content:
    on row 'i' a comma-separated list of minimal filtration values for data point 'i'.
    standard partial order on R^n.
  example:
    (0,0,1), (1,0,0)	// minima of point 1
    (1,1,1)	// minima of point 2
    ...
`
	flag.StringVar(&minima_file_name, "minima", "", aux_description)

	aux_description=`command line input of sub-filtration along which to compute 1d persistence.

  example:
    [VR_0, i_0, j_0, k_0,...]-- ... --[VR_n, i_n, j_n, k_n,...]
`
	flag.StringVar(&path_input, "path", "", aux_description)

	flag.StringVar(&threads, "threads", "", "number of threads (default: runtime.NumCPU())")
	flag.BoolVar(&verbose, "verbose", false, "Show status messages (default: false)")
	flag.BoolVar(&help, "help", false, "Show this help message")
	flag.BoolVar(&ripser, "ripser", false, "run ripser on auxiliary distance matrix (default: false)\n  Requires local ripser installation in PATH ")
	flag.StringVar(&ripser_dim, "dim", "1", "compute persistent homology up to dimension k (default: 1).")
	flag.StringVar(&ripser_threshold, "threshold", "", "compute persistent homology up to threshold t (in auxiliary distance matrix, default: enclosing radius).")
	flag.StringVar(&ripser_modulus, "modulus", "", "compute homology with coefficients in the prime field Z/pZ (default: 2).")
	flag.StringVar(&ripser_ratio, "ratio", "", "only show persistence pairs with death/birth ratio > r")


	// define custom flag.Usage() to be printed upon -help call.
	flag.Usage = func() {
			flagSet := flag.CommandLine
			aux_description = `Usage:
murit --dist <filename> --minima <filename> --path (VR_0, i_0, j_0, ...)-- ... --(VR_n, i_n, j_n, ...) [--options]
`
			fmt.Printf(aux_description)
			fmt.Printf("\nCommand Line Arguments\n")
			arguments := []string{"dist", "minima", "path", "threads", "verbose", "help", "ripser"}
			for _, name := range arguments {
					flag := flagSet.Lookup(name)
					fmt.Printf("-%s\n", flag.Name)
					fmt.Printf("  %s\n", flag.Usage)
			}
			ripser_arguments := []string{"dim", "threshold", "modulus", "ratio"}
			for _, name := range ripser_arguments {
					flag := flagSet.Lookup(name)
					fmt.Printf("	-%s\n", flag.Name)
					fmt.Printf("	  %s\n", flag.Usage)
			}
	}

	// Parse
	flag.Parse()

	// print help message
	if help {
		flag.Usage()
		return
	}

	// Check if required command line parameters are specified
	if dist_file_name == "" {
		log.Fatal("dist file name required (--dist_file)")
	}
	if minima_file_name == "" {
		log.Fatal("filtration file name required (--minima_file)")
	}


	// Read filtration file
	if verbose {fmt.Println("Pointwise minima")}
	minima_file, err := os.Open(minima_file_name)
	if err != nil {
		log.Fatalf("Failed to open file '%s': %v", minima_file_name, err)
	}

	var minima_list [][][]float64
	minima_scanner := bufio.NewScanner(minima_file)
	for minima_scanner.Scan() {
		var minima [][]float64
		// assume each line is a comma-separated list of minima in the format (a_1, a_2, ...), (b_1, b_2, ...)
		line := minima_scanner.Text()
		// split lines into separate minima (a_1, a_2, ...)
		for _, x := range strings.Split(line, "),("){
			// convert the separated minimum into a list of floats value, by value
			var minimum []float64
			for _, value_str := range strings.Split(strings.Trim(x, " ()"), ",") {
				value_float, err := strconv.ParseFloat(value_str, 64)
				if err != nil {
					log.Fatalf("Filtration parse error: %v", err)
				}
				minimum = append(minimum, value_float)
			}
			minima = append(minima, minimum)
		}
		minima_list = append(minima_list, minima)
	}
	if verbose {fmt.Println(minima_list)}
	// Close filtration file
	minima_file.Close()


  // Read sub filtration from command line OR create default sub filtration
	var path [][]float64
	if path_input != "" {
		// Parse sub filtration from command line input
		for _, p := range strings.Split(path_input,"--"){
			var point []float64
			for _, q := range strings.Split(strings.Trim(p, " []"),","){
				s, err := strconv.ParseFloat(q,64)
				if err != nil {
					log.Fatalf("Filtration parse error: %v", err)
				}
				point = append(point, s)
			}
			path = append(path, point)
		}
		// Check if sub filtration is valid (i.e. in lexicographical order)
		for i, j := 0, 1; j < len(path); i, j = i+1, j+1 {
			if !(leq(path[i], path[j])) {
				log.Fatalf("Invalid Path: path[%v] = %v !<= %v = path[%v]", i, path[i], path[j], j)
			}
		}
	} else {
		// Extract a valid sub filtration from the filtration file (traverse through list once and successively add larger elements)
		path = append(path, append([]float64{0}, minima_list[0][0]...))
		for _, x := range minima_list {
			if leq(path[len(path)-1], append([]float64{1}, x[0]...)) && !equal(path[len(path)-1], append([]float64{1}, x[0]...)) {
				path = append(path, append([]float64{1}, x[0]...))
			}
		}
	}
  if verbose {
		fmt.Println("\nPath")
		outer_sep := ""
		for _, fltr_point := range path {
			inner_sep := ""
			fmt.Print(outer_sep,"[")
			for _, value := range fltr_point{
				fmt.Print(inner_sep, value)
				inner_sep = ","
			}
			fmt.Print("]")
			outer_sep = "--"
		}
		fmt.Print("\n")
	}


	//
  // Prepare Input and Communication Channels
	//

	// In future development:
	// Set filename of auxiliary distance matrix in dependence of path
	// aux_file_name = filepath.Dir(dist_file_name)+"/"+hash(path_input)+".aux"
	aux_file_name = filepath.Dir(dist_file_name)+"/"+strconv.FormatInt(time.Now().UTC().UnixNano(), 10)+".aux"

  // Concatenate background information from above for workers
  b := args{path, minima_list}

  // Open distance matrix file
  in_file, err := os.Open(dist_file_name)
  if err != nil {
    log.Fatalf("Failed to open file '%s': %v", dist_file_name, err)
  }
  defer in_file.Close()   // defer closing of in_file until main() is closed

  // Initialize communication channels
  var numThreads int
	if threads == "" {
		numThreads = runtime.NumCPU()
		} else {
		numThreads, err = strconv.Atoi(threads)
		if err != nil {
			log.Fatalf("thread parsing error: %v", err)
		}
	}

	toWorker := make([]chan workload, numThreads)
	toWriter := make([]chan string, numThreads)
	for i := 0; i < numThreads; i++ {
		toWorker[i] = make(chan workload, 100)
		toWriter[i] = make(chan string, 100)
	}

  // Initialize buffered writer
  var out_writer *bufio.Writer
  aux_file, err := os.Create(aux_file_name)
  			if err != nil {
  				log.Fatalf("Failed to create file '%s': %v", aux_file_name, err)
  			}
  defer aux_file.Close()
  if ripser {
    out_writer = bufio.NewWriter(aux_file)
  } else {
		if verbose {fmt.Println("\nAuxiliary Distance Matrix")}
    out_writer = bufio.NewWriter(os.Stdout)
  }


	//
  // Parallel Execution
	//

	// Start reader
	go reader(in_file, toWorker)

	// Start workers
	for i := 0; i < numThreads; i++ {
		go worker(toWorker[i], toWriter[i], b)
	}

  // Start writer
	writer(toWriter, out_writer)



	//
	// Calculate Persistent Homology with Ripser
	//

	// Run ripser on auxiliary distance matrix
  var ripser_output []byte
	if ripser {
		if verbose {fmt.Println("\nRipser")}

		ripser_arguments := []string{"--format", "lower-distance"}
		if ripser_dim != ""{
			ripser_arguments = append(ripser_arguments, "--dim", ripser_dim)
		}
		if ripser_threshold != ""{
			ripser_arguments = append(ripser_arguments, "--threshold", ripser_threshold)
		}
		if ripser_modulus != ""{
			ripser_arguments = append(ripser_arguments, "--modulus", ripser_modulus)
		}
		if ripser_ratio != ""{
			ripser_arguments = append(ripser_arguments, "--ratio", ripser_ratio)
		}
		ripser_arguments = append(ripser_arguments, aux_file_name)

    ripser_cmd := exec.Command("ripser", ripser_arguments...)

    var err error
		ripser_output, err = ripser_cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to run command: %v\nCommand output: %s", err, string(ripser_output))
		}

	}

  err = os.Remove(aux_file_name)
  if err != nil {
		log.Fatalf("Failed to delete auxiliary distance matrix: %v", err)
	}

  // Translate ripser output of barcodes into barcodes on subfiltration
  print_flag := true
  scanner := bufio.NewScanner(strings.NewReader(string(ripser_output)))
  for scanner.Scan() {
    line := scanner.Text()

    if strings.Contains(line, "persistent homology") && strings.Contains(line, "dim 0") {
      print_flag = false
    } else if strings.Contains(line, "persistent homology") {
      print_flag = true
    }

    if print_flag {
      if strings.HasPrefix(line, " [") {
        line = strings.Trim(line, " [):")
        x := strings.Split(line, ",")

        birth, err := strconv.Atoi(x[0])
        if err != nil {
    			log.Fatalf("parsing error: %v", err)
    		}
				// shift from one- to zero-indexing
        birth = birth-1

        death, err := strconv.Atoi(x[1])
        if err != nil {
    			log.Fatalf("parsing error: %v", err)
    		}
				// shift from one- to zero-indexing
        death = death-1

        fmt.Println(" [", path[birth], ",", path[death], "):")
      } else {
        fmt.Println(line)
      }
    }
  }

}
