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

type workload struct {
  i    int
  text string
}

type args struct{
  sub_fltr [][]float64
  fltr_list [][]float64
}


// lexicographical comparisons of lists (of integers)
func leq(poset_element1 []float64, poset_element2 []float64) bool {
	var L int
	if len(poset_element1) <= len(poset_element2) {
		L = len(poset_element1)
	} else {
		L = len(poset_element2)
	}
	for a:= 0; a<L; a++ {
		if (poset_element1[a] > poset_element2[a]) {
			return false
		}
	}
	return true
}

func equal(poset_element1 []float64, poset_element2 []float64) bool {
	var L int
	if len(poset_element1) <= len(poset_element2) {
		L = len(poset_element1)
	} else {
		L = len(poset_element2)
	}
	for a:= 0; a<L; a++ {
		if (poset_element1[a] != poset_element2[a]) {
			return false
		}
	}
	return true
}


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
      // Goal: Determine modified distance for pair of datapoints (x_i,x_j), see article.
      // find sub-filtration point in which the given edge is present and set distance to that value.
      modified_distance := len(b.sub_fltr)
      fltr_point_i := append([]float64{distance}, b.fltr_list[w.i]...)
      fltr_point_j := append([]float64{distance}, b.fltr_list[j]...)
      for k, x := range b.sub_fltr {
        if leq(fltr_point_i, x) && leq(fltr_point_j, x) {
          modified_distance = k
          break
        }
      }
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



func main() {
	var dist_file_name string
	var fltr_file_name string
	var threads string
	var sub_fltr_input string
  var aux_file_name string

	var ripser bool
	var verbose bool
	var help bool

	// Parse command line options
	flag.StringVar(&dist_file_name, "dist_file", "", "distance file name")
	flag.StringVar(&fltr_file_name, "fltr_file", "", "filtration file name. Each row gives filtration value of corresponding data point.\n Format i,j,k,... (interpreted with lexicraphical order)")
	flag.StringVar(&threads, "threads", "", "number of threads")
	flag.StringVar(&sub_fltr_input, "sub_fltr", "", "sub-filtration along which to compute 1d persistence.\n Format: [VR_0, i_0, j_0, k_0,...]-- ... --(VR_n, i_n, j_n, k_n,...)")

	flag.BoolVar(&ripser, "ripser", false, "run ripser?")
	flag.BoolVar(&verbose, "verbose", false, "show status messages?")
	flag.BoolVar(&help, "help", false, "Get help message")
	flag.Parse()

	// print help message
	if help {
		flag.PrintDefaults()
		return
	}

	// Check if required command line parameters are specified
	if dist_file_name == "" {
		log.Fatal("dist file name required (--dist_file)")
	}
	if fltr_file_name == "" {
		log.Fatal("filtration file name required (--fltr_file)")
	}


	if verbose {fmt.Println("---")}
	// Read filtration file
	if verbose {fmt.Println("Read point annotation file")}
	fltr_file, err := os.Open(fltr_file_name)
	if err != nil {
		log.Fatalf("Failed to open file '%s': %v", fltr_file_name, err)
	}

	var fltr_list [][]float64
	fltr_scanner := bufio.NewScanner(fltr_file)
	for fltr_scanner.Scan() {
		var row []float64
			for _, v := range strings.Split(fltr_scanner.Text(),",") {
				s, err := strconv.ParseFloat(v,64)
				if err != nil {
					log.Fatalf("Filtration parse error: %v", err)
				}
				row = append(row, s)
		}
		fltr_list = append(fltr_list, row)
	}
	// Close filtration file
	fltr_file.Close()


  // Read sub filtration from command line OR create default sub filtration
	if verbose {fmt.Println("Read subfiltration")}
	var sub_fltr [][]float64
	if sub_fltr_input != "" {
		// Parse sub filtration from command line input
		for _, p := range strings.Split(sub_fltr_input,"--"){
			var point []float64
			for _, q := range strings.Split(strings.Trim(p, " []"),","){
				s, err := strconv.ParseFloat(q,64)
				if err != nil {
					log.Fatalf("Filtration parse error: %v", err)
				}
				point = append(point, s)
			}
			sub_fltr = append(sub_fltr, point)
		}
		// Check if sub filtration is valid (i.e. in lexicographical order)
		for i, j := 0, 1; j < len(sub_fltr); i, j = i+1, j+1 {
			if !(leq(sub_fltr[i], sub_fltr[j])) {
				log.Fatalf("Invalid sub-filtration: sub_fltr[%v] = %v !<= %v = sub_fltr[%v]", i, sub_fltr[i], sub_fltr[j], j)
			}
		}
    // Set filename of auxiliary distance matrix in dependence of sub_fltr
    aux_file_name = filepath.Dir(dist_file_name)+"/"+hash(sub_fltr_input)+".aux"
	} else {
		// Extract a valid sub filtration from the filtration file (traverse through list once and successively add larger elements)
		sub_fltr = append(sub_fltr, append([]float64{0}, fltr_list[0]...))
		for _, x := range fltr_list {
			x = append([]float64{1}, x...)
			if leq(sub_fltr[len(sub_fltr)-1], x) && !equal(sub_fltr[len(sub_fltr)-1], x) {
				sub_fltr = append(sub_fltr, x)
			}
		}
    aux_file_name = filepath.Dir(dist_file_name)+"/"+strconv.FormatInt(time.Now().UTC().UnixNano(), 10)+".aux"
	}


  if verbose {fmt.Println(sub_fltr)}


  //// Prepare input and communication channels
	if verbose {fmt.Println("Building auxiliary Distance Matrix")}

  // Concatenate background information from above for workers
  b := args{sub_fltr, fltr_list}

  // Open distance matrix file
  in_file, err := os.Open(dist_file_name)
  if err != nil {
    log.Fatalf("Failed to open file '%s': %v", dist_file_name, err)
  }
  defer in_file.Close()   // defer closing of in_file (MB: defers until main is closed, I think)

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
    out_writer = bufio.NewWriter(os.Stdout)
  }


  //// Parallel execution

	// Start reader
	go reader(in_file, toWorker)

	// Start workers
	for i := 0; i < numThreads; i++ {
		go worker(toWorker[i], toWriter[i], b)
	}

  // Start writer
	writer(toWriter, out_writer)


	//// Run ripser on auxiliary distance matrix and translate result
  var ripser_output []byte
	if ripser {
		if verbose {fmt.Println("---")}
		if verbose {fmt.Println("Run Ripser")}
    ripser_cmd := exec.Command("ripser", "--format", "lower-distance", aux_file_name)
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
        birth = birth-1

        death, err := strconv.Atoi(x[1])
        if err != nil {
    			log.Fatalf("parsing error: %v", err)
    		}
        death = death-1

        fmt.Println(" [", sub_fltr[birth], ",", sub_fltr[death], "):")
      } else {
        fmt.Println(line)
      }
    }
  }

}
