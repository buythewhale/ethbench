package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os/user"
	"path"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	concurrency, number, to, socketPath, from := parseArgs()

	socket, _ := net.Dial("unix", socketPath)
	defer socket.Close()
	runBenchmark(socket, from, to, number, concurrency)
}
func parseArgs() (concurrency int, number int, to string, socketPath string, from string) {
	_concurrency := flag.Int("concurrency", 10, "Number of concurrently sent requests")
	_number := flag.Int("number", 100, "Total number of requests to send")
	_to := flag.String("to", "0x00E3d1Aa965aAfd61217635E5f99f7c1e567978f", "Recipient address")
	_socketPath := flag.String("socket", getDefaultSocketPath(), "IPC socket path")
	flag.Parse()
	_from := flag.Arg(0)
	return *_concurrency, *_number, *_to, *_socketPath, _from
}
func runBenchmark(socket net.Conn, from string, to string, number int, concurrency int) {
	startedAt := time.Now()
	var failed int32 = 0
	tasks := make(chan int, number)
	for i := 0; i < number; i++ {
		tasks <- i
	}
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			for range tasks {
				if !sendEther(socket, from, to) {
					atomic.AddInt32(&failed, 1)
				}
			}
			wg.Done()
		}()
	}
	close(tasks)
	wg.Wait()
	took := time.Since(startedAt)
	succeed := number - int(failed)
	log.Printf(
		"%v succeeded, %v failed, success rate %.2f",
		succeed,
		failed,
		float64(succeed)/float64(number),
	)
	log.Printf("took %v, %.2f tx/s total, %.2f tx/s successful",
		took,
		float64(number)/took.Seconds(),
		float64(succeed)/took.Seconds(),
	)
}

func sendEther(socket net.Conn, from string, to string) bool {
	request := fmt.Sprintf(
		`{
			"jsonrpc":"2.0",
			"method":"personal_sendTransaction",
			"params":[{"from":"%v","to":"%v","gas":"0x5208","gasPrice":"0x4a817c800","value":"0xde0b6b3a7640000"},""],"id":1}`,
		from,
		to)

	socket.Write([]byte(request))
	buff := make([]byte, 102)
	_, err := socket.Read(buff)

	if err != nil {
		return false
	}

	return strings.Contains(string(buff), `"result":"0x`)
}

func getDefaultSocketPath() string {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	var relativePath string
	switch runtime.GOOS {
	case "darwin":
		relativePath = "Library/Application Support/io.parity.ethereum/jsonrpc.ipc"
	default:
		relativePath = ".local/share/io.parity.ethereum/jsonrpc.ipc"
	}

	return path.Join(usr.HomeDir, relativePath)
}
