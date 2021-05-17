package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

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

func bfsWorker(input chan string, output chan string, action func(path string, info os.FileInfo), wg *sync.WaitGroup) {
	for path := range input {
		info, err := os.Stat(path)
		if err != nil {
			log.Fatal(err)
		}
		action(path, info)

		if info.IsDir() {
			files, err := ioutil.ReadDir(path)
			if err != nil {
				log.Fatal(err)
			}
			for _, file := range files {
				output <- filepath.Join(path, file.Name())
			}
		}
	}
	wg.Done()
}

func bfs(root string, action func(path string, info os.FileInfo)) {
	curLvlData := []string{root}
	threadCount := runtime.NumCPU()
	for len(curLvlData) > 0 {
		var wgWorkers sync.WaitGroup
		curLvlChan := make(chan string, threadCount)
		stringArrAsChan(curLvlData, curLvlChan)
		nextLvlChan := make(chan string, threadCount)
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
	bfs("./", func(path string, info os.FileInfo) {
		fmt.Println(path)
	})
}