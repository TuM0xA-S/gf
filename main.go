package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
)

type bfsCallback func(path string, info os.FileInfo) error

func stringArrAsChan(arr []string, ch chan string) {
	go func() {
		for _, val := range arr {
			ch <- val
		}
		close(ch)
	}()
}

func stringChanAsArr(ch chan string, res *[]string, signal chan bool) {
	for val := range ch {
		*res = append(*res, val)
	}
	signal <- true
}

func bfsWorker(input chan string, output chan string, action bfsCallback, wg *sync.WaitGroup) {
	for path := range input {
		info, err := os.Lstat(path)
		if err != nil {
			log.Println(err)
			continue
		}

		if action(path, info) == filepath.SkipDir {
			continue
		}

		if info.Mode().Type() == fs.ModeSymlink {
			var err error
			info, err = os.Stat(path)
			if err != nil {
				log.Println(err)
				continue
			}
		}

		if info.IsDir() {
			files, err := ioutil.ReadDir(path)
			if err != nil {
				log.Println(err)
				continue
			}
			for _, file := range files {
				output <- filepath.Join(path, file.Name())
			}
		}
	}
	wg.Done()
}

func bfs(roots []string, action bfsCallback) {
	curLvlData := roots
	threadCount := 4 * runtime.NumCPU()
	for len(curLvlData) > 0 {
		var wgWorkers sync.WaitGroup
		curLvlChan := make(chan string, 16*threadCount)
		stringArrAsChan(curLvlData, curLvlChan)
		nextLvlChan := make(chan string, 16*threadCount)
		for i := 0; i < threadCount; i++ {
			wgWorkers.Add(1)
			go bfsWorker(curLvlChan, nextLvlChan, action, &wgWorkers)
		}
		nextLvlData := []string(nil)
		signal := make(chan bool)
		go stringChanAsArr(nextLvlChan, &nextLvlData, signal)
		wgWorkers.Wait()
		close(nextLvlChan)
		<-signal
		curLvlData = nextLvlData
	}
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("ERROR: ")
	logWriter := bufio.NewWriter(os.Stderr)
	log.SetOutput(logWriter)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "gf [<options>...] <pattern> [<directores>...]")
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(), "  -h\tshow this help")
	}

	fullPath := flag.Bool("p", false, "show full path")
	followSymlinks := flag.Bool("f", false, "follow symlinks")
	quietMode := flag.Bool("q", false, "quiet mode")
	matchCount := flag.Int("n", math.MaxInt32, "max results count")
	flag.Parse()

	if *matchCount == 0 {
		return
	}

	if *quietMode {
		log.SetOutput(ioutil.Discard)
	}

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	regex, err := regexp.Compile(args[0])
	if err != nil {
		log.Println(err)
		os.Exit(2)
	}

	directories := []string{"./"}
	if len(args) > 1 {
		directories = args[1:]
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Println(err)
		os.Exit(1)
		return
	}

	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
		}
	}()

	output := make(chan string, 1024)
	signal := printer(output, *matchCount)
	go func() {
		<-signal
		os.Exit(0)
	}()
	bfs(directories, func(path string, info os.FileInfo) error {
		name := filepath.Base(path)
		if *fullPath {
			path = filepath.Join(cwd, path)
		}
		if regex.MatchString(name) {
			output <- path
		}

		if info.Mode().Type() == fs.ModeSymlink && !*followSymlinks {
			return filepath.SkipDir
		}

		return nil
	})
	close(output)
	<-signal
}

func printer(ch chan string, count int) chan bool {
	writer := bufio.NewWriter(os.Stdout)
	signal := make(chan bool)
	currentCnt := 0
	go func() {
		defer func() {
			writer.Flush()
			signal <- true
		}()
		for str := range ch {
			writer.WriteString(str + "\n")
			currentCnt++
			if currentCnt >= count {
				return
			}
		}
	}()
	return signal
}
