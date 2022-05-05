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




func main() {
	// const time_layout = "2006-01-02"
	var dist_file_name string
	var fltr_file_name string
	var write_to_file_name string
	var threads string
	var sub_fltr_input string

	var compressed_dist_file bool
	var compress_out_file bool
	var ripser bool
	var debug bool
	var help bool

	// Parse command line options
	flag.StringVar(&dist_file_name, "dist_file", "", "distance file name")
	flag.StringVar(&fltr_file_name, "fltr_file", "", "filtration file name. Each row gives filtration value of corresponding data point.\n Format i,j,k,... (interpreted with lexicraphical order)")
	flag.StringVar(&write_to_file_name, "write_to", "", "write modified distance to specified file")
	flag.StringVar(&threads, "threads", "", "number of threads")
	flag.StringVar(&sub_fltr_input, "sub_fltr", "", "sub-filtration along which to compute 1d persistence.\n Format: [r0,i0,j0,k0,...]-- ... --(rn,in,jn,kn,...)")

	flag.BoolVar(&compressed_dist_file, "compressed_dist_file", false, "Is distance file compressed?")
	flag.BoolVar(&compress_out_file, "compress_out_file", false, "Compress timedist file?")
	flag.BoolVar(&ripser, "ripser", false, "Run ripser?")
	flag.BoolVar(&debug, "debug", false, "print some messages for debugging purposes?")
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
	if write_to_file_name == "" {
		log.Fatal("output file name (currently) required (--write_to)")
	}
	if sub_fltr_input == "" {
		log.Fatal("sub-filtration is required (--sub_fltr)")
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
	// Close ids file
	fltr_file.Close()


	var sub_fltr [][]int
	for _, p := range strings.Split(sub_fltr_input,"--"){
		var point []int
		for _, q := range strings.Split(strings.Trim(p, "[]"),","){
			s, err := strconv.Atoi(q)
			if err != nil {
				log.Fatalf("Filtration parse error: %v", err)
			}
			point = append(point, s)
		}
		sub_fltr = append(sub_fltr, point)
	}


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

	if debug == true {
	fmt.Println("filtration list", fltr_list)
	fmt.Println("sub filtration", sub_fltr)
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
				// calculate deformation for pair of datapoints (w.i,j), see article.
				// reminder: sub_fltr_idx[0] is the Rips parameter
				D := len(sub_fltr)
				for k, sub_fltr_idx := range sub_fltr {
					cond1 := false
					for a:= 0; a<len(fltr_list[i]); a++ {
						if (fltr_list[i][a] <= sub_fltr_idx[a+1]) {
							cond1 = true
							break
						}
					}
					cond2 := false
					for a:= 0; a<len(fltr_list[i]); a++ {
						if fltr_list[j][a] <= sub_fltr_idx[a+1] {
							cond2 = true
							break
						}
					}

					if (cond1 || cond2) {
						D = k
						break
					}

				}

				var deformation int
				for _, sub_fltr_idx:= range sub_fltr[0:D] {
						deformation += sub_fltr_idx[0]
				}
				modified_distance = distance + deformation
				if debug == true {
					fmt.Println(i, j, ":", "neither is in filtr. step(s)", sub_fltr[0:D], "-> D=", D, "deformation=", deformation, "\n", "dist(", i, ",", j, ")=", distance, "-> mod_dist(", i, ",", j, ")=", modified_distance)
				}
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
			out_file, err := os.Create(write_to_file_name)
			if err != nil {
				log.Fatalf("Failed to create file '%s': %v", write_to_file_name, err)
			}
			defer out_file.Close()
			out_writer = bufio.NewWriter(out_file)
		} else {
			// Command to execute
			cmd := exec.Command("zstd", "-T0", "--force", "--quiet", "-", "-o", write_to_file_name)

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
		var max_deformation_int int
		for _, sub_fltr_idx:= range sub_fltr {
				max_deformation_int += sub_fltr_idx[0]
		}
		if debug == true {
		fmt.Println(max_deformation_int)
		}
		max_deformation_str := strconv.Itoa(max_deformation_int)
		cmd := exec.Command("ripser", write_to_file_name, "--threshold", max_deformation_str)
		cmd.Wait()
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Failed to run command: %v\nCommand output: %s", err, string(output))
		}
		fmt.Println(string(output))
	}
}
