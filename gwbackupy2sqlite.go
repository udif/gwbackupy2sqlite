package main

import (
	"bufio"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/mail"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	//_ "github.com/mattn/go-sqlite3"
	_ "github.com/glebarez/go-sqlite"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var wg sync.WaitGroup
var wg2 sync.WaitGroup

var fileMap = make(map[int64]string)
var fileMu sync.Mutex

//var fileMap = &SafeMap{
//	m: make(map[int64]string),
//}

// There is no built-in Max() function for integers
func Max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

//type SafeMap struct {
//	mu sync.Mutex
//	m  map[int64]string
//}
//
//func (sm *SafeMap) safeSet(key int64, value string) {
//	sm.mu.Lock()
//	sm.m[key] = value
//	sm.mu.Unlock()
//}
//
//func (sm *SafeMap) safeGet(key int64) (string, bool) {
//	sm.mu.Lock()
//	defer sm.mu.Unlock()
//	val, ok := sm.m[key]
//	return val, ok
//}

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

func handleMail(goroutineNum int, filename string, resultCh chan<- string) {
	var gzName, jsonName string
	dir, file := filepath.Split(filename)
	id, err := strconv.ParseInt(file[0:16], 16, 64)
	if err != nil {
		panic(err)
	}
	fmt.Println(id)
	ext := filepath.Ext(filename)
	fileMu.Lock() // release lock ASAP
	val, ok := fileMap[id]
	if !ok {
		// We got json, but if no map hit, we only got 1 file yet
		fileMap[id] = filename // save 1st filename
		fileMu.Unlock()        // release lock ASAP
		return                 // return, wait for next file
	} else {
		fileMu.Unlock() // got both file, release lock ASAP
	}
	// we got both filenames in "filename" and "val", now determine which is which
	ext2 := filepath.Ext(val)
	if ext == ".json" {
		jsonName = filename
		gzName = val
		if ext2 != ".gz" {
			log.Fatal("ext == json but ext2 != gz")
		}
	} else {
		jsonName = val
		gzName = filename
		if ext != ".gz" {
			log.Fatal("ext != gz and ext != json", ext)
		}
		if ext2 != ".json" {
			log.Fatal("ext == gz but ext2 != json")
		}
	}
	//fmt.Println("jsonName:", jsonName, "gzName:", gzName)
	dir2, _ := filepath.Split(filename)
	if dir != dir2 {
		log.Fatal("dir != dir2")
	}
	// optional sanity checks:
	// both filename has the same id
	// both filenames resides in the same directory
	// we reall have json and gz suffixes
	dir3, day := filepath.Split(filepath.Clean(dir))
	_, year := filepath.Split(filepath.Clean(dir3))
	fmt.Println("gzip: year:", year, "day:", day, "id:", id)

	var dec = new(mime.WordDecoder)
	dec.CharsetReader = CharsetReader
	// Handle json file (metadata)
	byteValue, err := os.ReadFile(jsonName)
	if err != nil {
		log.Fatal(err)
	}
	var result map[string]interface{}
	json.Unmarshal([]byte(byteValue), &result)
	fmt.Println(result)

	// Handle gz file (email payload)
	fhg, err := os.Open(gzName)
	if err != nil {
		log.Fatal(err)
	}
	defer fhg.Close()

	gz, err := gzip.NewReader(fhg)
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
	if &decodedSubject == &encodedSubject {
		// If both vars point to the same strings, it means no new buffer was allocated,
		// and no quote-printable string ('=?') was found in encodedSubject
		decodedSubject = convertRawToUTF8(encodedSubject)
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	modifiedSubject := fmt.Sprintf("%s: %s : %d", "" /*filename*/, decodedSubject, goroutineNum)
	resultCh <- modifiedSubject
}

func workerFunc(goroutineNum int, fileCh <-chan string, resultCh chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()

	for filename := range fileCh {
		handleMail(goroutineNum, filename, resultCh)
	}
}

func sqlite_update(resultCh <-chan string) {
	defer wg2.Done()

	for result := range resultCh {
		fmt.Println(result)
	}
}

type SemVer struct {
	major int
	minor int
	patch int
}

func (v *SemVer) LessThan(v2 *SemVer) (res bool) {
	if v.major < v2.major {
		return true
	} else if v.major > v2.major {
		return false
	} else if v.minor < v2.minor {
		return true
	} else if v.minor > v2.minor {
		return false
	} else if v.patch < v2.patch {
		return true
	} else {
		return false
	}
}

func (v *SemVer) Equal(v2 *SemVer) (res bool) {
	if v.major == v2.major &&
		v.minor == v2.minor &&
		v.patch == v2.patch {
		return true
	} else {
		return false
	}
}

func (v *SemVer) GreaterThan(v2 *SemVer) (res bool) {
	if v.major > v2.major {
		return true
	} else if v.major < v2.major {
		return false
	} else if v.minor > v2.minor {
		return true
	} else if v.minor < v2.minor {
		return false
	} else if v.patch > v2.patch {
		return true
	} else {
		return false
	}
}

// To be filled as the DB progresses
func upgradeDb(db *sql.DB, dbv *SemVer) {
	//
}

func openDatabase(db_name string) (db *sql.DB, err error) {
	db, err = sql.Open("sqlite", db_name)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	var table string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='schema_version';").Scan(&table)

	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No DB found, creating a new one.")
			// read SQL commands from file
			sqlCommands, err := os.ReadFile("tables.sql")
			if err != nil {
				log.Fatal(err)
			}
			// execute SQL commands
			_, err = db.Exec(string(sqlCommands))
			if err != nil {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	} else {
		cv := SemVer{major: 1, minor: 0, patch: 0}
		var dbv SemVer
		err = db.QueryRow("SELECT * FROM schema_version;").Scan(&dbv.major, &dbv.minor, &dbv.patch)
		fmt.Println("gabackupy2sqlite DB Version:", dbv.major, dbv.minor, dbv.patch)
		if cv.GreaterThan(&dbv) {
			upgradeDb(db, &dbv)
		}
	}
	return db, err
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
	openDatabase(*db)
	os.Exit(0)

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
