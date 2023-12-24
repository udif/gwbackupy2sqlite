package main

import (
	"bufio"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/mail"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var wg sync.WaitGroup
var wg2 sync.WaitGroup

// There is no built-in Max() function for integers
func Max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

func CharsetReader(charset string, input io.Reader) (io.Reader, error) {
	var dec *encoding.Decoder
	switch strings.ToLower(charset) {
	case "iso-8859-8", "iso-8859-8-i":
		// Replace with the actual ISO-8859-8 decoder
		dec = charmap.ISO8859_8.NewDecoder()
	case "windows-1255":
		// Replace with the actual Windows-1255 decoder
		dec = charmap.Windows1255.NewDecoder()
	case "gb18030", "gb2312":
		dec = simplifiedchinese.GB18030.NewDecoder()
	case "koi8-r":
		dec = charmap.KOI8R.NewDecoder()
	default:
		return nil, fmt.Errorf("unknown charset: %s", charset)
	}
	return transform.NewReader(input, dec), nil
}

// Convert raw strings to UTF-8
// check for specific encoding using heuristics
// add here more as needed
// At the moment, we assume it is ISO-8859-8 or ISO-8859-8-I
// https://en.wikipedia.org/wiki/ISO/IEC_8859-8
func convertRawToUTF8(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0xe0 && s[i] <= 0xfa {
			// Must be Hebrew
			dec := charmap.ISO8859_8.NewDecoder()
			res, err := dec.String(s)
			if err != nil {
				fmt.Println("Error decoding string:", err)
				return s
			}
			return res
		}
	}
	return s
}

func handleGzip(goroutineNum int, filename string, resultCh chan<- string) {
	var dec = new(mime.WordDecoder)
	dec.CharsetReader = CharsetReader

	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}

	gz, err := gzip.NewReader(file)
	if err != nil {
		log.Fatal(err)
	}
	defer gz.Close()

	scanner := bufio.NewScanner(gz)
	var headers string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			break
		}
		headers += line + "\r\n"
	}
	//fmt.Println(headers)
	msg, err := mail.ReadMessage(strings.NewReader(headers))
	if err != nil {
		log.Fatal(err)
	}

	encodedSubject := msg.Header.Get("Subject")
	decodedSubject, err := dec.DecodeHeader(encodedSubject)
	if decodedSubject == encodedSubject {
		// If both strings equal, it means no new buffer was allocated,
		// and no quote-printable string ('=?') was found in encodedSubject
		decodedSubject = convertRawToUTF8(encodedSubject)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	modifiedSubject := fmt.Sprintf("%s: %s : %d", "" /*filename*/, decodedSubject, goroutineNum)
	resultCh <- modifiedSubject
}

func handleJson(goroutineNum int, filename string, resultCh chan<- string) {

}
func workerFunc(goroutineNum int, fileCh <-chan string, resultCh chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()

	for filename := range fileCh {
		if strings.HasSuffix(filename, ".gz") {
			handleGzip(goroutineNum, filename, resultCh)
		} else if strings.HasSuffix(filename, ".json") {
			handleJson(goroutineNum, filename, resultCh)
		}
	}
}

func sqlite_update(resultCh <-chan string) {
	defer wg2.Done()

	for result := range resultCh {
		fmt.Println(result)
	}
}

func main() {
	dir := flag.String("dir", "", "directory path")
	db := flag.String("db", "", "database name")
	numProcs := flag.Int("procs", Max(1, runtime.NumCPU()-2), "number of parallel processes")
	flag.Parse()
	if *dir == "" || *db == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	var files []string
	err := filepath.Walk(*dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		fmt.Println(err)
		return
	}

	fileCh := make(chan string)
	resultCh := make(chan string)

	// This goroutine receives the modified filenames and prints them.
	wg2.Add(1)
	go sqlite_update(resultCh)

	// Start 8 goroutines.
	for i := 0; i < *numProcs; i++ {
		wg.Add(1)
		go workerFunc(i, fileCh, resultCh, &wg)
	}

	// Send each filename to a goroutine.
	for _, filename := range files {
		if strings.HasSuffix(filename, ".gz") {
			fileCh <- filename
		}
		if strings.HasSuffix(filename, ".json") {
			fileCh <- filename
		}
	}

	close(fileCh)
	wg.Wait()
	close(resultCh)
	wg2.Wait()

}
