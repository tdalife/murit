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
	// "time"
)


func leq(poset_element1 []int, poset_element2 []int) bool {
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

func equal(poset_element1 []int, poset_element2 []int) bool {
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


func main() {
	var dist_file_name string
	var fltr_file_name string
	var to_file string
	var threads string
	var sub_fltr_input string

	var compressed_dist_file bool
	var compress_out_file bool
	var ripser bool
	var debug bool
	var verbose bool
	var help bool

	// Parse command line options
	flag.StringVar(&dist_file_name, "dist_file", "", "distance file name")
	flag.StringVar(&fltr_file_name, "fltr_file", "", "filtration file name. Each row gives filtration value of corresponding data point.\n Format i,j,k,... (interpreted with lexicraphical order)")
	flag.StringVar(&to_file, "to_file", "", "write modified distance to specified file")
	flag.StringVar(&threads, "threads", "", "number of threads")
	flag.StringVar(&sub_fltr_input, "sub_fltr", "", "sub-filtration along which to compute 1d persistence.\n Format: [VR_0, i_0, j_0, k_0,...]-- ... --(VR_n, i_n, j_n, k_n,...)")

	flag.BoolVar(&compressed_dist_file, "compressed_dist_file", false, "Is distance file compressed?")
	flag.BoolVar(&compress_out_file, "compress_out_file", false, "Compress timedist file?")
	flag.BoolVar(&ripser, "ripser", false, "run ripser?")
	flag.BoolVar(&debug, "debug", false, "show messages for debugging purposes?")
	flag.BoolVar(&verbose, "verbose", false, "show status messages?")
	flag.BoolVar(&help, "help", false, "Get help message")
	flag.Parse()

	// print help message
	if help {
		flag.PrintDefaults()
		return
	}

	// Check command line parameters
	if dist_file_name == "" {
		log.Fatal("dist file name required (--dist_file)")
	}
	if fltr_file_name == "" {
		log.Fatal("filtration file name required (--fltr_file)")
	}
	if to_file == "" {
		log.Fatal("output file name (currently) required (--to_file)")
	}


	// Read filtration file
	fltr_file, err := os.Open(fltr_file_name)
	if err != nil {
		log.Fatalf("Failed to open file '%s': %v", fltr_file_name, err)
	}

	var fltr_list [][]int
	fltr_scanner := bufio.NewScanner(fltr_file)
	for fltr_scanner.Scan() {
		var row []int
			for _, v := range strings.Split(fltr_scanner.Text(),",") {
				s, err := strconv.Atoi(v)
				if err != nil {
					log.Fatalf("Filtration parse error: %v", err)
				}
				row = append(row, s)
		}
		fltr_list = append(fltr_list, row)
	}
	// Close filtration file
	fltr_file.Close()


	var sub_fltr [][]int
	if sub_fltr_input != "" {
		// Parse sub filtration from command line input
		for _, p := range strings.Split(sub_fltr_input,"--"){
			var point []int
			for _, q := range strings.Split(strings.Trim(p, " []"),","){
				s, err := strconv.Atoi(q)
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
	} else {
		// Extract a valid sub filtration from the filtration file (traverse through list once and successively add larger elements)
		sub_fltr = append(sub_fltr, append([]int{0}, fltr_list[0]...))
		for _, x := range fltr_list {
			x = append([]int{1}, x...)
			if leq(sub_fltr[len(sub_fltr)-1], x) && !equal(sub_fltr[len(sub_fltr)-1], x) {
				sub_fltr = append(sub_fltr, x)
			}
		}
	}


	var max_deformation int
	for _, x := range sub_fltr {
			max_deformation += x[0]
	}
	max_deformation++

	ripser_threshold := 2*max_deformation



	// ToDo: allow floats as filtration values
	// ToDo: implement automatic conversion from date-isostring to filtration, e.g. via unix seconds
	// Note: Will also need to convert the given path from isostring into unix seconds
	// // Compute time differences in days to start_date
	// var time_diff []int
	// ids_scanner := bufio.NewScanner(ids_file)
	// for ids_scanner.Scan() {
	// 	t, err := time.Parse(
	// 		time_layout,
	// 		strings.Split(ids_scanner.Text(), "|")[2])
	// 	if err != nil {
	// 		log.Fatalf("Time parse error: %v", err)
	// 	}
	// 	time_diff =
	// 		append(time_diff,
	// 			int(t.Sub(start_date).Hours()/24))
	// }

	if (verbose == true) || (debug == true) {
	fmt.Println("filtration list", fltr_list)
	fmt.Println("sub filtration", sub_fltr)
	fmt.Println("max possible deformation =",max_deformation)
	fmt.Println("ripser threshold =", ripser_threshold)
	}

	type workload struct {
		i    int
		text string
	}

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

	// Reader function
	reader := func(out []chan workload) {
		// read in file
		var in_scanner *bufio.Scanner
		if !compressed_dist_file {
			in_file, err := os.Open(dist_file_name)
			if err != nil {
				log.Fatalf("Failed to open file '%s': %v", dist_file_name, err)
			}
			// Close in_file
			defer in_file.Close()

			in_scanner = bufio.NewScanner(in_file)
		} else {
			// Command to execute
			cmd := exec.Command("zstd", "-T0", "--decompress", "--quiet", dist_file_name, "--stdout")

			// Connect command stdin to in_scanner
			inPipe, err := cmd.StdoutPipe()
			if err != nil {
				log.Fatalf("Failed to create decompress pipe: %v", err)
			}
			in_scanner = bufio.NewScanner(inPipe)

			// Start command
			err = cmd.Start()
			if err != nil {
				output, _ := cmd.CombinedOutput()
				log.Fatalf("Failed to run command: %v\nCommand output: %s", err, string(output))
			}

			// Wait for command to finish
			defer cmd.Wait()
			// Close pipe
			defer inPipe.Close()
		}

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
	worker := func(in chan workload, out chan string) {
		var sb strings.Builder

		for w := range in {
			// clear string builder for new matrix line
			sb.Reset()
			i := w.i

			// Split line of distance matrix into the single distances at separator
			splitLine := strings.Split(w.text, ",")

			for j, token := range splitLine {
				var modified_distance int
				// convert distance string to int
				distance, err := strconv.Atoi(token)
				if err != nil {
					log.Fatalf("Distance conversion error: %v", err)
				}
				// Goal: Determine deformation for pair of datapoints (x_i,x_j), see article.
				deformation := 0
				if distance != 0 {
					// 1st step. find max(D(x),D(y)) ~ point in the subfiltration from where on both x and y are contained.
					both_in_sub_fltr := false
					var D int
					for k, x := range sub_fltr {
						if leq(fltr_list[i], x[1:]) && leq(fltr_list[j], x[1:]) {
							D = k
							both_in_sub_fltr = true
							break
						}
					}
					// 2nd step. Determine deformation value, depends on distance(x,y)
					// reminder: sub_fltr[x][0] is the Vietoris-Rips parameter of the xth filtration value in the sub filtration
					d := D
					if both_in_sub_fltr {
						for sub_fltr[d][0] == 0 && d < len(sub_fltr)-1 {
							d++
						}
						if (distance <= sub_fltr[d][0]) {
							for _, x:= range sub_fltr[0:d] {
								deformation += x[0]
							}
						} else {
							deformation = max_deformation
						}
					} else {
						deformation = ripser_threshold
					}
					if debug == true {
						fmt.Println("points (",i,",", j,")", "not both contained in filtr. step(s)", sub_fltr[0:D], "-> D=", D, "; dist(", i, ",", j, ")=", distance, "sub_fltr[d][0]=", sub_fltr[d][0], "dist <= sub_fltr[d][0] ", distance<=sub_fltr[d][0], "; deformation=", deformation, "=>", "mod_dist(", i, ",", j, ")=", distance+deformation)
					}
				}
				// 3rd step. calculate modified distance
				modified_distance = distance + deformation
				sb.WriteString(strconv.Itoa(modified_distance)) // write modified distance
				sb.WriteByte(',')                               // write separator
			}
			// Send modified matrix line to writer
			joinLine := sb.String()
			out <- joinLine[:len(joinLine)-1]
		}

		// Close Channel
		close(out)
	}

	// Writer function
	writer := func(in []chan string) {

		var out_writer *bufio.Writer
		if !compress_out_file {
			// read in file
			out_file, err := os.Create(to_file)
			if err != nil {
				log.Fatalf("Failed to create file '%s': %v", to_file, err)
			}
			defer out_file.Close()
			out_writer = bufio.NewWriter(out_file)
		} else {
			// Command to execute
			cmd := exec.Command("zstd", "-T0", "--force", "--quiet", "-", "-o", to_file)

			// Connect command stdin to out_writer
			outPipe, err := cmd.StdinPipe()
			if err != nil {
				log.Fatalf("Failed to create pipe: %v", err)
			}
			out_writer = bufio.NewWriter(outPipe)

			// Start command
			err = cmd.Start()
			if err != nil {
				output, _ := cmd.CombinedOutput()
				log.Fatalf("Failed to run command: %v\nCommand output: %s", err, string(output))
			}

			// Wait for command to finish
			defer cmd.Wait()
			// Close pipe
			defer outPipe.Close()
		}

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

				// Write line and new line to file
				_, err := out_writer.WriteString(line)
				if err != nil {
					log.Fatalf("Failed to write to out_writer: %v", err)
				}
				_, err = out_writer.WriteString("\n")
				if err != nil {
					log.Fatalf("Failed to write to out_writer: %v", err)
				}
			}
		}
	}

	// Setup parallel execution:

	// Start reader
	go reader(toWorker)

	// Start workers
	for i := 0; i < numThreads; i++ {
		go worker(toWorker[i], toWriter[i])
	}

	// Start writer
	writer(toWriter)

	if ripser {
		cmd := exec.Command("ripser", "--format", "lower-distance", "--threshold", strconv.Itoa(ripser_threshold), to_file)
		// _ = ripser_threshold
		// cmd := exec.Command("ripser", "--format", "lower-distance", to_file)
		// cmd.Wait()
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to run command: %v\nCommand output: %s", err, string(output))
		}
		fmt.Println(string(output))
	}
}
