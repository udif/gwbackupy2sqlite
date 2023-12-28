package main

import (
	"strconv"
	"strings"
)

type Int64Hex int64

func (i *Int64Hex) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	v, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return err
	}
	*i = Int64Hex(v)
	return nil
}

type Int64Str int64

func (i *Int64Str) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*i = Int64Str(v)
	return nil
}

type IntStr int

func (i *IntStr) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	v, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	*i = IntStr(v)
	return nil
}

// While the LabelIds field appears here, it will NOT be written into the emails table in the database
// It is here only to let us read it with the rest of the JSON data in one pass.
type Emails struct {
	Id           Int64Hex `json:"id"`
	ThreadId     Int64Hex `json:"threadId"`
	InternalDate Int64Str `json:"internalDate"`
	LabelIds     []string `json:"labelIds"`
	Subject_e    string
	Snippet      string `json:"snippet"`
	HistoryId    IntStr `json:"historyId"`
	SizeEstimate IntStr `json:"sizeEstimate"`
	Date_e       int64
}
