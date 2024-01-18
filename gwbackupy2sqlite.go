package main

import (
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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	//_ "github.com/mattn/go-sqlite3"
	_ "github.com/glebarez/go-sqlite"
	"github.com/udif/yaccDate/yaccDate"
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
	case "iso-8859-8", "iso-8859-8-i", "unknown":
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

var logger *log.Logger

var dateLayout1 = []string{
	"Mon, 2 Jan 2006 15:4:5 -0700",
	"_2 Jan 2006 15:4:5 -0700",
	"Mon, 2 Jan 06 15:4:5 -0700",
	"Mon, 2 Jan 2006 15:4:5",
	"Mon, 2 Jan 2006 15:4:5 MST-0700",
	"Mon, 2 Jan 2006 15:4:5 MST:00",
	"existent_DATE_TIME",
	""}
var dateLayout2 = []string{
	"Mon, 2 Jan 2006 15:4:5 -0700 (MST)",
	"Mon, 2 Jan 2006 15:4:5 -0700 MST",
	"Mon, 2 Jan 2006 15:4:5 MST",
	"Mon, 2-Jan-2006 15:4:5 MST",
	"Mon, 2 Jan 2006 15:4:5 \"MST\"",
	"_2 Jan 2006 15:4:5 MST",
	", Mon,, 2-Jan-2006, 15:4:5, MST",
	"Mon, 2 Jan 06 15:4:5 MST"}

func parseDateMultiple(dateStr string, layouts []string) (time.Time, []error) {
	var errors []error
	var err error
	var t time.Time
	for _, format := range layouts {
		t, err = time.Parse(format, dateStr)
		if err == nil {
			return t, nil
		}
		errors = append(errors, err)
	}
	return t, errors
}

func printErrors(errors []error) {
	for _, err := range errors {
		log.Println(err.Error())
	}
}

func parseDateFlexible(dateStr string) time.Time {
	var errors1, errors2, errors3 []error
	var t time.Time
	t, errors1 = parseDateMultiple(dateStr, dateLayout1)
	if len(errors1) == 0 {
		return t
	}
	t, errors2 = parseDateMultiple(dateStr, dateLayout2)
	if len(errors2) == 0 {
		return t
	}
	dateStr = strings.Replace(dateStr, "UT", "UTC", 1)
	t, errors3 = parseDateMultiple(dateStr, dateLayout2)
	if len(errors3) == 0 {
		return t
	}
	printErrors(errors1)
	printErrors(errors2)
	printErrors(errors3)
	os.Exit(1)
	return t
}

// var tzones = make(map[string]bool)
var tzones sync.Map

func canonizeDateFormat(input string) string {
	input2 := strings.ToLower(input)
	weekdays_months := map[string]string{
		"monday": "sunday", "tuesday": "sunday", "wednesday": "sunday", "thursday": "sunday", "friday": "sunday", "saturday": "sunday",
		"mon": "sun", "tue": "sun", "wed": "sun", "thu": "sun", "fri": "sun", "sat": "sun",
		"february": "january", "march": "january", "april": "january", "may": "january", "june": "january", "july": "january", "august": "january", "september": "january", "october": "january", "november": "january", "december": "january",
		"feb": "jan", "mar": "jan", "apr": "jan", "jun": "jan", "jul": "jan", "aug": "jan", "sep": "jan", "oct": "jan", "nov": "jan", "dec": "jan",
	}

	for k, v := range weekdays_months {
		input2 = strings.ReplaceAll(input2, k, v)
	}

	re := regexp.MustCompile(`\b[a-z]{3}\b`)
	matches := re.FindAllString(input2, -1)
	for _, match := range matches {
		m := strings.ToLower(match)
		if m != "jan" && m != "sun" && m != "gmt" && m != "utc" {
			_, exists := tzones.Load(match)
			if !exists {
				tzones.Store(match, true)
			}
			input2 = strings.ReplaceAll(input2, match, "XXX")
		}
	}

	re = regexp.MustCompile(`\d`)
	input2 = re.ReplaceAllString(input2, "0")

	return "## " + input + " => " + input2
}

func handleMail(goroutineNum int, filename string, resultCh chan<- string) {
	var gzName, jsonName string
	_, file := filepath.Split(filename)
	dir := filepath.Dir(filename)
	id, err := strconv.ParseInt(file[0:16], 16, 64)
	if err != nil {
		fmt.Println("Filename:", filename)
		panic(err)
	}
	//fmt.Println(id)
	ext := filepath.Ext(filename)
	fileMu.Lock() // release lock ASAP
	val, ok := fileMap[id]
	dir2 := filepath.Dir(val)
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
			fmt.Println(filename, val)
			logger.Fatal("ext == json but ext2 != gz")
		}
	} else {
		jsonName = val
		gzName = filename
		if ext != ".gz" {
			logger.Fatal("ext != gz and ext != json", ext)
		}
		if ext2 != ".json" {
			logger.Fatal("ext == gz but ext2 != json")
		}
	}
	//fmt.Println("jsonName:", jsonName, "gzName:", gzName)
	if dir != dir2 {
		logger.Fatal("dir != dir2")
	}
	// optional sanity checks:
	// both filename has the same id
	// both filenames resides in the same directory
	// we reall have json and gz suffixes
	//dir3, day := filepath.Split(filepath.Clean(dir))
	//_, year := filepath.Split(filepath.Clean(dir3))
	//fmt.Println("gzip: year:", year, "day:", day, "id:", id)
	month_day := filepath.Base(dir2)
	year := filepath.Base(filepath.Dir(dir2))

	var dec = new(mime.WordDecoder)
	dec.CharsetReader = CharsetReader

	// ***************************
	// Handle JSON file (metadata)
	// ***************************
	byteValue, err := os.ReadFile(jsonName)
	if err != nil {
		logger.Fatal(err)
	}
	var email Emails
	json.Unmarshal([]byte(byteValue), &email)
	// ********************
	// End of JSON handling
	// ********************

	// Handle gz file (email payload)
	fhg, err := os.Open(gzName)
	if err != nil {
		logger.Fatal(err)
	}
	defer fhg.Close()

	gz, err := gzip.NewReader(fhg)
	if err != nil {
		logger.Fatal(err)
	}
	defer gz.Close()

	msg, err := mail.ReadMessage(gz)
	if err != nil {
		if len(filename) > 0 {
			logger.Println("Filename:", filename)
		} else {
			logger.Println("nil Filename!")
		}
		logger.Println(err)
		return
	}
	encodedDate := msg.Header.Get("Date")
	fmt.Println(canonizeDateFormat(encodedDate))
	return

	var chat bool = false
	var decodedSubject string
	for _, v := range email.LabelIds {
		if v == "CHAT" {
			chat = true
		}
	}
	if !chat {
		encodedSubject := msg.Header.Get("Subject")
		decodedSubject, err := dec.DecodeHeader(encodedSubject)
		if &decodedSubject == &encodedSubject {
			// If both vars point to the same strings, it means no new buffer was allocated,
			// and no quote-printable string ('=?') was found in encodedSubject
			decodedSubject = convertRawToUTF8(encodedSubject)
		}
		//fmt.Println(msg.Header.Get("Date"))
		encodedDate := msg.Header.Get("Date")
		t, err := yaccDate.FlexDateToTime(encodedDate) // used to be time.RFC1123Z
		idate := int64(email.InternalDate) / 1000
		if err != nil {
			fmt.Printf("%s/%s/%016x: Date='%s' Error:%s\n", year, month_day, id, encodedDate, err)
			email.Date_e = idate
			os.Exit(1)
		} else {
			decodedDate, _ := dec.DecodeHeader(encodedDate)
			if &decodedDate == &encodedDate {
				// If both vars point to the same strings, it means no new buffer was allocated,
				// and no quote-printable string ('=?') was found in encodedSubject
				decodedDate = convertRawToUTF8(encodedDate)
			}
			email.Date_e = t.Unix()
			if email.Date_e != idate {
				fmt.Println(t)
				fmt.Printf("%s/%s/%016x: Date='%s' vs '%s', %d != %d\n", year, month_day, id, decodedDate, time.Unix(idate, 0), email.Date_e, idate)
				os.Exit(1)
			} else {
				fmt.Printf("%s/%s/%016x: Date='%s'\n", year, month_day, id, encodedDate)
			}
		}
	} else {
		decodedSubject = "Chat with " // + TODO
		// gmail derives it from datetime of last chat message, translated to your local TZ
		// dates in chat are encoded as GMT+0
		email.Date_e = int64(email.InternalDate) / 1000
	}
	email.Subject_e = decodedSubject
	//fmt.Println(email)

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
		_ = result
		//fmt.Println(result)
	}
}

// To be filled as the DB progresses
func upgradeDb(db *sql.DB, dbv *SemVer) {
	//
}

func openDatabase(db_name string) (db *sql.DB, err error) {
	db, err = sql.Open("sqlite", db_name)
	if err != nil {
		logger.Fatal(err)
	}

	var table string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='schema_version';").Scan(&table)

	if err != nil {
		if err == sql.ErrNoRows {
			fmt.Println("No DB found, creating a new one.")
			// read SQL commands from file
			sqlCommands, err := os.ReadFile("tables.sql")
			if err != nil {
				logger.Fatal(err)
			}
			// execute SQL commands
			_, err = db.Exec(string(sqlCommands))
			if err != nil {
				logger.Fatal(err)
			}
		} else {
			logger.Fatal(err)
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
	db_name := flag.String("db", "", "database name")
	logname := flag.String("log", "", "logfile name")
	numProcs := flag.Int("procs", Max(1, runtime.NumCPU()-2), "number of parallel processes")
	flag.Parse()
	if *dir == "" || *db_name == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}
	if *logname != "" {
		file, err := os.OpenFile(*logname, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		logger = log.New(file, "", log.LstdFlags)
	}
	var files []string
	/*
		years, err := os.ReadDir(*dir)
		if err != nil {
			logger.Fatal(err)
		}

		for _, year := range years {
			if year.IsDir() {
				match, _ := regexp.MatchString(`^\d{4}$`, s)
				if !match {
					continue
				}
				year_dir := filepath.Join(*dir, year.Name())
				if
				month_days, err := os.ReadDir(year_dir)
				if err != nil {
					logger.Fatal(err)
				}
				for _, month_day := range month_days {
					month_day_dir := filepath.Join(year_dir, month_day.Name())
					if month_day.IsDir() {
						match, _ = regexp.MatchString(`^\d{2}-\d{2}$`, s)
						fmt.Println(month_day_dir)
					}
				}
			}
		}
		os.Exit(0)
	*/
	re := regexp.MustCompile(`\d{4}/\d{2}-\d{2}/[0-9A-Fa-f]{16}[^/]*$`)
	err := filepath.Walk(*dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			path2 := filepath.ToSlash(path)
			if match := re.MatchString(path2); match {
				files = append(files, path)
			}
		}
		return nil
	})

	if err != nil {
		fmt.Println(err)
		return
	}
	db, err := openDatabase(*db_name)
	defer db.Close()
	//os.Exit(0)

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
		//fmt.Println(filename)
		if strings.HasSuffix(filename, ".gz") {
			fileCh <- filename
		}
		if strings.HasSuffix(filename, ".json") {
			fileCh <- filename
		}
	}

	fmt.Println(tzones)
	close(fileCh)
	wg.Wait()
	close(resultCh)
	wg2.Wait()

}
