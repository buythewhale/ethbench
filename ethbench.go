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
	"sync/atomic"
	"time"
	"os"
	"os/signal"
)

var succeeded int32
var failed int32
var startedAt = time.Now()

func main() {
	concurrency, duration, to, endpoints, accounts := parseArgs()
	if len(endpoints) != len(accounts) {
		log.Printf("Number of endpoints (%v) doesn't match number of accounts (%v)", len(endpoints), len(accounts))
		return
	}

	for i := 0; i < len(endpoints); i++ {
		socket, err := net.Dial("unix", endpoints[i])
		if err != nil {
			log.Println(err)
			return
		}
		//noinspection GoDeferInLoop
		defer socket.Close()

		go runBenchmark(socket, accounts[i], to, concurrency)
	}

	go printThroughput()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			printResults()
			os.Exit(0)
		}
	}()

	time.Sleep(duration)

	printResults()
}
func printThroughput() {
	last := succeeded
	ticker := time.NewTicker(1 * time.Second)
	for {
		select {
		case <-ticker.C:
			sentPerSec := succeeded - last
			last += sentPerSec
			log.Printf("%v tx/s", sentPerSec)
			// do stuff
		}
	}
}
func printResults() {
	fmt.Println()
	took := time.Since(startedAt)
	total := succeeded + failed
	log.Printf(
		"%v succeeded, %v failed, success rate %.2f",
		succeeded,
		failed,
		float64(succeeded)/float64(total),
	)
	log.Printf("test time %v, %.2f tx/s total, %.2f tx/s successful",
		took,
		float64(total)/took.Seconds(),
		float64(succeeded)/took.Seconds(),
	)
}

func parseArgs() (concurrency int, duration time.Duration, to string, endpoints []string, accounts []string) {
	_concurrency := flag.Int("concurrency", 10, "Number of concurrently succeeded requests")
	_duration := flag.Int64("duration", (1<<63-1)/int64(time.Second), "Test duration in seconds")
	_to := flag.String("to", "0x00E3d1Aa965aAfd61217635E5f99f7c1e567978f", "Recipient address")
	_endpoints := flag.String("endpoints", getDefaultSocketPath(), "IPC socket paths")
	flag.Parse()
	_from := flag.Arg(0)
	return *_concurrency, time.Duration(*_duration * int64(time.Second)), *_to, strings.Split(*_endpoints, ","), strings.Split(_from, ",")
}

func runBenchmark(socket net.Conn, from string, to string, concurrency int) {
	tasks := make(chan int)

	for i := 0; i < concurrency; i++ {
		go func() {
			for range tasks {
				if !sendEther(socket, from, to) {
					atomic.AddInt32(&failed, 1)
				} else {
					atomic.AddInt32(&succeeded, 1)
				}
			}
		}()
	}
	for i := 0; ; i++ {
		tasks <- i
	}
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
