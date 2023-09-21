// MIT License
//
// (C) Copyright [2018-2021,2023] Hewlett Packard Enterprise Development LP
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the "Software"),
// to deal in the Software without restriction, including without limitation
// the rights to use, copy, modify, merge, publish, distribute, sublicense,
// and/or sell copies of the Software, and to permit persons to whom the
// Software is furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR
// OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.

package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/Cray-HPE/hms-base"
	"github.com/Cray-HPE/hms-hmetcd"
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var SMRMutex sync.Mutex
var SMRval int = http.StatusOK
var router *mux.Router

// Capture buffer for our roll-your-own fmt.Printf/Println functions
var testStdout string
var tdMutex sync.Mutex

// Home-brewed replacements for fmt.Printf and fmt.Println.  These will
// capture the stdout and use it to be able to compare output from hbtd
// functions.
//
// fmtstr(in):  Printf format string
// a(in):       Variadic parameters
// Return:      int: 0 (assume success), nil: assume no error

func testPrintf(fmtstr string, a ...interface{}) {
	tdMutex.Lock()
	//Extract data as needed?
	tdata := fmt.Sprintf(fmtstr, a...)
	//Insure newline
	td := strings.TrimSpace(tdata)
	td += "\n"
	//Append to an array
	testStdout = testStdout + td
	tdMutex.Unlock()
}

func testPrintln(a ...interface{}) {
	tdMutex.Lock()
	//Extract data as needed?
	tdata := fmt.Sprintln(a...)
	//Append to an array
	//Insure newline
	td := strings.TrimSpace(tdata)
	td += "\n"
	testStdout = testStdout + td
	tdMutex.Unlock()
}

func testPrintData() string {
	var rstr string
	tdMutex.Lock()
	rstr = testStdout
	tdMutex.Unlock()
	return rstr
}

func testPrintClear() {
	tdMutex.Lock()
	testStdout = ""
	tdMutex.Unlock()
}

var is_setup int = 0

func one_time_setup() error {
	var err error = nil

	if is_setup == 0 {
		kvHandle, err = hmetcd.Open("mem:", "")
	}

	return err
}

// Compares expected hb_checker() stdout with actual.  Actual stdout
// is split by newline into separate strings, which will match the
// expected stdout format.  Both sets of strings are sorted, and then
// compared.  Note that the sorting is necessary since hb_checker()
// iterates over a hash, and order is never guaranteed.
//
// exps(in):  Expected stdout strings.
// acts(in):  Actual stdout of hb_checker, newline separated.
// Return:    0 if things compare OK, -1 if not

func hb_compare(exps []string, acts string) int {
	var loc_acts []string

	//Sort the exps with the same also as sorting the actuals

	loc_exps := exps
	sort.Strings(loc_exps)

	//Split the actuals into strings at the newlines and filter out SM patch
	//errors, which are irrelevant.

	la := strings.Split(strings.TrimSuffix(acts, "\n"), "\n")
	for ix, _ := range la {
		if strings.Contains(la[ix], "ERROR sending PATCH") {
			continue
		}
		loc_acts = append(loc_acts, la[ix])
	}
	sort.Strings(loc_acts)

	if len(loc_exps) != len(loc_acts) {
		return -1
	}

	rx := regexp.MustCompile("overdue [0-9]+ seconds")
	ntimeStr := "overdue xx seconds"
	var estr, astr string

	for ix := 0; ix < len(loc_exps); ix++ {
		//Remove all specific mention of seconds due to overly quantized
		//time delay measurement.

		estr = rx.ReplaceAllString(loc_exps[ix], ntimeStr)
		astr = rx.ReplaceAllString(loc_acts[ix], ntimeStr)
		if estr != astr {
			return -1
		}
	}
	return 0
}

func make_key(val *string, key string, ts int64) {
	*val = fmt.Sprintf("{\"Component\":\"%s\",\"Last_hb_rcv_time\":\"%x\",\"Last_hb_timestamp\":\"\",\"Last_hb_status\":\"OK\",\"Had_warning\":\"%s\"}",
		key, ts, HB_WARN_NONE)
}

// Test entry point for hb_checker().
//
// This will directly call hb_checker, not relying on the timer.  We may
// want to test the timer too, but for now just call directly.
//
// t(in):  Test framework.
// Return: None.

func TestHb_checker(t *testing.T) {
	var err error
	var kval string
	var tpd string
	var exp_strs_1 = []string{ //after 4 sec
		`Number of components heartbeating: 3`,
	}

	var exp_strs_2 = []string{ //after 7 sec
		`WARNING: Heartbeat overdue 7 seconds for 'x0c1s2b0n3' (might be dead), last status: 'OK'`,
		//`WARNING: Heartbeat overdue 5 seconds for 'x1c2s3b0n4' (might be dead), last status: 'OK'`,
	}

	var exp_strs_3 = []string{ //after 15 sec
		//`ERROR: Heartbeat overdue 15 seconds for 'x0c1s2b0n3' (declared dead), last status: 'OK'`,
		`WARNING: Heartbeat overdue 10 seconds for 'x1c2s3b0n4' (might be dead), last status: 'OK'`,
		//`WARNING: Heartbeat overdue 9 seconds for 'x2c3s4b0n5' (might be dead), last status: 'OK'`,
		//`WARNING: Heartbeat overdue 8 seconds for 'x3c4s5b0n6' (might be dead), last status: 'OK'`,
		//`Number of components heartbeating: 1`,
	}

	var exp_strs_4 = []string{ //after 30 sec
		`ERROR: Heartbeat overdue 30 seconds for 'x0c1s2b0n3' (declared dead), last status: 'OK'`,
		`ERROR: Heartbeat overdue 25 seconds for 'x1c2s3b0n4' (declared dead), last status: 'OK'`,
		`ERROR: Heartbeat overdue 20 seconds for 'x2c3s4b0n5' (declared dead), last status: 'OK'`,
		`Number of components heartbeating: 0`,
	}

	// Set up the app_params to specify warning and error HB timeouts

	t.Logf("** RUNNING hb_checker TEST **")

	ots_err := one_time_setup()
	if ots_err != nil {
		t.Error("ERROR setting up KV store:", ots_err)
		return
	}
	staleKeys = false
	app_params.debug_level.int_param = 0
	app_params.check_interval.int_param = 0
	app_params.warntime.int_param = 5
	app_params.errtime.int_param = 20

	// Create  KV entries for test components.  Note that this would also
	// be tested if we mock up an HTTP request and call hb_rcv()

	basetime := time.Now().Unix()

	keys := []string{"x0c1s2b0n3", "x1c2s3b0n4", "x2c3s4b0n5"}
	dlys := []int{0, 5, 12}

	for ix, key := range keys {
		make_key(&kval, key, (basetime + int64(dlys[ix])))
		err = kvHandle.Store(key, kval)
		if err != nil {
			t.Error("ERROR creating KV record for '", key, "': ", err)
		}
	}

	t.Logf("** Testing HB checker timer loop **\n")

	// Sleep for various amounts of time, calling hb_checker() after each
	// sleep cycle.  Heartbeat stops will be printed.

	// Test 1: 4 seconds elapsed. Should be no HB stops

	testPrintClear()
	t.Logf("Sleeping 4 secs.\n")
	time.Sleep(4 * time.Second)
	t.Logf("Invoking HB checker.\n")
	hb_checker()
	time.Sleep(100 * time.Millisecond)
	tpd = testPrintData()
	if hb_compare(exp_strs_1, tpd) != 0 {
		t.Errorf("ERROR, mismatch\nExp: '%s'\nAct: '%s'\n\n",
			exp_strs_1, tpd)
	}

	//Test 2: 7 seconds elapsed.  The first 1 should show a HB stop warning

	testPrintClear()
	t.Logf("Sleeping 3 secs.\n")
	time.Sleep(3 * time.Second)
	t.Logf("Invoking HB checker.\n")
	hb_checker()
	time.Sleep(100 * time.Millisecond)
	tpd = testPrintData()
	if hb_compare(exp_strs_2, tpd) != 0 {
		t.Errorf("ERROR, mismatch\nExp: '%s'\nAct: '%s'\n\n",
			exp_strs_2, tpd)
	}

	//Test 3: 15 seconds elapsed.
	//The second one should show a HB stop warning

	testPrintClear()
	t.Logf("Sleeping 8 secs.\n")
	time.Sleep(8 * time.Second)
	t.Logf("Invoking HB checker.\n")
	hb_checker()
	time.Sleep(100 * time.Millisecond)
	tpd = testPrintData()
	if hb_compare(exp_strs_3, tpd) != 0 {
		t.Errorf("ERROR, mismatch\nExp: '%s'\nAct: '%s'\n\n",
			exp_strs_3, tpd)
	}

	//Test 4: 30 seconds elapsed
	//Remaining ones should show HB stop errors

	testPrintClear()
	t.Logf("Sleeping 22 secs.\n")
	time.Sleep(22 * time.Second)
	t.Logf("Invoking HB checker.\n")
	hb_checker()
	time.Sleep(100 * time.Millisecond)
	tpd = testPrintData()
	if hb_compare(exp_strs_4, tpd) != 0 {
		t.Errorf("ERROR, mismatch\nExp: '%s'\nAct: '%s'\n\n",
			exp_strs_4, tpd)
	}

	time.Sleep(2 * time.Second)
	t.Logf("  ==> FINISHED hb_checker TEST.")
}

func heartbeat(comp string) error {
	kval, ok, err := kvHandle.Get(comp)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("Key '%s' does not exist.", comp)
	}
	var hbb hbinfo
	umerr := json.Unmarshal([]byte(kval), &hbb)
	if umerr != nil {
		return umerr
	}

	hbb.Last_hb_rcv_time = strconv.FormatUint(uint64(time.Now().Unix()), 16)
	if hbb.Had_warning == HB_WARN_GAP {
		hbb.Had_warning = HB_WARN_NORMAL
	}
	jstr, jerr := json.Marshal(hbb)
	if jerr != nil {
		return jerr
	}

	err = kvHandle.Store(comp, string(jstr))
	if err != nil {
		return err
	}
	return nil
}

func kill_sm_goroutines() {
	for {
		if len(hsmUpdateQ) == 0 {
			break
		}
		_ = <-hsmUpdateQ
	}
	hsmUpdateQ <- HSMQ_DIE
	time.Sleep(100 * time.Millisecond)
	for {
		if len(hsmUpdateQ) == 0 {
			break
		}
		_ = <-hsmUpdateQ
	}

	time.Sleep(2 * time.Second)
}

// Similar to TestHb_checker(), but this version checks for stale keys

func TestHb_checker2(t *testing.T) {
	var err error
	var kval string
	var tpd string

	var exp_strs_1 = []string{
		`WARNING: Heartbeat overdue 60 seconds for 'x0c1s2b0n3' due to HB monitoring gap; might be dead, last status: 'OK'`,
		`WARNING: Heartbeat overdue 60 seconds for 'x0c1s2b0n4' due to HB monitoring gap; might be dead, last status: 'OK'`,
		`Number of components heartbeating: 2`,
	}

	var exp_strs_2 = []string{`INFO: Heartbeat restarted for 'x0c1s2b0n3'`}

	var exp_strs_3 = []string{}

	var exp_strs_4 = []string{
		`ERROR: Heartbeat overdue 24 seconds for 'x0c1s2b0n4' (declared dead), last status: 'OK'`,
		`Number of components heartbeating: 1`,
	}

	//TODO: do we need a time second edge detect?

	hbtdPrintf = testPrintf
	hbtdPrintln = testPrintln

	t.Logf("** RUNNING hb_checker GAP TEST **")

	ots_err := one_time_setup()
	if ots_err != nil {
		t.Error("ERROR setting up KV store:", ots_err)
		return
	}
	hsmReady = true
	staleKeys = true
	app_params.debug_level.int_param = 0
	app_params.check_interval.int_param = 0
	app_params.warntime.int_param = 15
	app_params.errtime.int_param = 20

	for len(hsmUpdateQ) > 0 {
		<-hsmUpdateQ
	}

	go send_sm_req()
	time.Sleep(500 * time.Millisecond)

	// Create  KV entries for test components.  Note that this would also
	// be tested if we mock up an HTTP request and call hb_rcv()

	basetime := time.Now().Unix()

	key1 := "x0c1s2b0n3"
	make_key(&kval, key1, (basetime - int64(60)))
	err = kvHandle.Store(key1, kval)
	if err != nil {
		t.Error("ERROR creating KV record for '", key1, "': ", err)
	}
	key2 := "x0c1s2b0n4"
	make_key(&kval, key2, (basetime - int64(60)))
	err = kvHandle.Store(key2, kval)
	if err != nil {
		t.Error("ERROR creating KV record for '", key2, "': ", err)
	}

	testPrintClear()
	hb_checker()
	time.Sleep(100 * time.Millisecond)
	tpd = testPrintData()
	if hb_compare(exp_strs_1, tpd) != 0 {
		t.Errorf("ERROR, mismatch\nExp: '%s'\nAct: '%s'\n\n",
			exp_strs_1, tpd)
	}
	// At this time, both keys' times are set to base

	time.Sleep(2 * time.Second)

	//"heartbeat" by updating a node's time stamp
	err = heartbeat(key1)
	if err != nil {
		t.Errorf("ERROR performing fake heartbeat: %v", err)
	}

	//comp key1 HB is set to base+2.
	testPrintClear()
	time.Sleep(10 * time.Second)

	//Should see restarted warning on key1
	hb_checker()
	time.Sleep(100 * time.Millisecond)
	tpd = testPrintData()
	if hb_compare(exp_strs_2, tpd) != 0 {
		t.Errorf("ERROR, mismatch\nExp: '%s'\nAct: '%s'\n\n",
			exp_strs_2, tpd)
	}

	heartbeat(key1)

	testPrintClear()
	time.Sleep(4 * time.Second)
	//This is base + 17 for key2.  Should not show any warnings.

	hb_checker()
	time.Sleep(100 * time.Millisecond)
	tpd = testPrintData()
	if hb_compare(exp_strs_3, tpd) != 0 {
		t.Errorf("ERROR, mismatch\nExp: '%s'\nAct: '%s'\n\n",
			exp_strs_3, tpd)
	}

	testPrintClear()
	time.Sleep(6 * time.Second)
	//This is base + 24.  Should be an error for key2

	heartbeat(key1)
	hb_checker()
	time.Sleep(100 * time.Millisecond)
	tpd = testPrintData()
	if hb_compare(exp_strs_4, tpd) != 0 {
		t.Errorf("ERROR, mismatch\nExp: '%s'\nAct: '%s'\n\n",
			exp_strs_4, tpd)
	}

	//Kill the send goroutine

	kill_sm_goroutines()
}

func hb_cmp(t *testing.T, cmp string, ts string, status string) {
	var kval string
	var kok bool
	var err error
	var kjson hbinfo

	//Fetch key from KV store

	kval, kok, err = kvHandle.Get(cmp)
	if err != nil {
		t.Error("ERROR looking up key '", cmp, "': ", err)
		return
	}
	if !kok {
		t.Errorf("ERROR, key not found: '%s'\n", cmp)
		return
	}

	err = json.Unmarshal([]byte(kval), &kjson)
	if err != nil {
		t.Error("ERROR unmarshalling '", kval, "': ", err)
		return
	}

	if kjson.Component != cmp {
		t.Errorf("ERROR, mismatch component name in '%s', got '%s'.\n",
			cmp, kjson.Component)
		return
	}
	if kjson.Last_hb_timestamp != ts {
		t.Errorf("ERROR, mismatch timestamp in '%s', got '%s'.\n",
			ts, kjson.Last_hb_timestamp)
		return
	}
	if kjson.Last_hb_status != status {
		t.Errorf("ERROR, mismatch status in '%s', got '%s'.\n",
			status, kjson.Last_hb_status)
		return
	}
}

// Test entry point for hb_rcv(), which is the HB HTTP request handler.
// We will fake out an HTTP request object and record the response.
// We will also examine the newly-created components in the HB tracking
// map to be sure the right ones were created and with the right info.
//
// t(in)   Test framework.
// Return: None.

func TestHb_rcv(t *testing.T) {
	t.Logf("** RUNNING HEARTBEAT HTTP OPERATIONS TEST\n")

	ots_err := one_time_setup()
	if ots_err != nil {
		t.Error("ERROR setting up KV store:", ots_err)
		return
	}

	//Slop together JSON payloads for 2 heartbeating nodes

	req1_hb := bytes.NewBufferString(`{"Component":"x1c2s2b0n3","Hostname":"nid0001.us.cray.com","NID":"0001","Status":"OK","Timestamp":"Jan 1, 0000"}`)
	req2_hb := bytes.NewBufferString(`{"Status":"OK","Timestamp":"Jan 2, 1000"}`)

	// Create 2 fake HTTP POSTs with the HB data in it
	req1, err1 := http.NewRequest("POST", "http://localhost:8080/hmi/v1/heartbeat", req1_hb)

	if err1 != nil {
		t.Fatal(err1)
	}

	req2, err2 := http.NewRequest("POST", "http://localhost:8080/hmi/v1/heartbeat/x2c3s4b0n5", req2_hb)

	if err2 != nil {
		t.Fatal(err2)
	}

	// Set up to grab the "responses"
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hbRcv)
	handlerXName := http.HandlerFunc(hbRcvXName)

	// Mock up the first operation
	handler.ServeHTTP(rr, req1)

	// Check the return code
	if rr.Code != http.StatusOK {
		t.Errorf("HTTP handler returned bad error code, got %v, want %v",
			rr.Code, http.StatusOK)
	}

	// Mock up the second operation
	handlerXName.ServeHTTP(rr, req2)

	// Check the return code
	if rr.Code != http.StatusOK {
		t.Errorf("HTTP handler returned bad error code, got %v, want %v",
			rr.Code, http.StatusOK)
	}

	//Now check the hbmap to see if it has entries for both nodes

	hb_cmp(t, "x1c2s2b0n3", "Jan 1, 0000", "OK")
	hb_cmp(t, "x2c3s4b0n5", "Jan 2, 1000", "OK")

	//Now check some error conditions.  First, a non-POST request

	req_e1, err_e1 := http.NewRequest("GET", "http://localhost:8080/hmi/v1/heartbeat", nil)

	if err_e1 != nil {
		t.Fatal(err_e1)
	}

	rr_e1 := httptest.NewRecorder()
	handler_e1 := http.HandlerFunc(hbRcv)
	handler_e1.ServeHTTP(rr_e1, req_e1)

	// Check the return code
	if rr_e1.Code != http.StatusMethodNotAllowed {
		t.Errorf("HTTP handler returned bad error code, got %v, want %v",
			rr_e1.Code, http.StatusMethodNotAllowed)
	}

	//Next we'll give it JSON with an invalid data type

	req_e1_data := bytes.NewBufferString(`{"Component":1234,"Hostname":"nid0001.us.cray.com","NID":"0001","Status":"OK","Timestamp":"Jan 1, 0000"}`)
	req_e1, err_e1 = http.NewRequest("POST", "http://localhost:8080/hmi/v1/heartbeat", req_e1_data)

	if err_e1 != nil {
		t.Fatal(err_e1)
	}
	rr_e1 = httptest.NewRecorder()
	handler_e1 = http.HandlerFunc(hbRcv)
	handler_e1.ServeHTTP(rr_e1, req_e1)

	// Check the return code
	if rr_e1.Code != http.StatusBadRequest {
		t.Errorf("HTTP handler returned bad error code, got %v, want %v",
			rr_e1.Code, http.StatusBadRequest)
	}

	//Send HB with a missing field.

	req_e1_data = bytes.NewBufferString(`{"Hostname":"nid0001.us.cray.com","NID":"0001","Status":"OK","Timestamp":"Jan 1, 0000"}`)
	req_e1, err_e1 = http.NewRequest("POST", "http://localhost:8080/hmi/v1/heartbeat", req_e1_data)

	if err_e1 != nil {
		t.Fatal(err_e1)
	}
	rr_e1 = httptest.NewRecorder()
	handler_e1 = http.HandlerFunc(hbRcv)
	handler_e1.ServeHTTP(rr_e1, req_e1)

	// Check the return code
	if rr_e1.Code != http.StatusBadRequest {
		t.Errorf("HTTP handler returned bad error code, got %v, want %v",
			rr_e1.Code, http.StatusBadRequest)
	}

	//Send a HB with an invalid component XName

	req_e1_data = bytes.NewBufferString(`{"Component":"xxyyzz","Hostname":"nid0001.us.cray.com","NID":"0001","Status":"OK","Timestamp":"Jan 1, 0000"}`)
	req_e1, err_e1 = http.NewRequest("POST", "http://localhost:8080/hmi/v1/heartbeat", req_e1_data)

	if err_e1 != nil {
		t.Fatal(err_e1)
	}
	rr_e1 = httptest.NewRecorder()
	handler_e1 = http.HandlerFunc(hbRcv)
	handler_e1.ServeHTTP(rr_e1, req_e1)

	// Check the return code
	if rr_e1.Code != http.StatusBadRequest {
		t.Errorf("HTTP handler returned bad error code, got %v, want %v",
			rr_e1.Code, http.StatusBadRequest)
	}

	//Send a HB with a NID that's numerically invalid

	req_e1_data = bytes.NewBufferString(`{"Component":"x0c0s0b0n0","Hostname":"nid0001.us.cray.com","NID":"123456789123456789123456789123456789","Status":"OK","Timestamp":"Jan 1, 0000"}`)
	req_e1, err_e1 = http.NewRequest("POST", "http://localhost:8080/hmi/v1/heartbeat", req_e1_data)

	if err_e1 != nil {
		t.Fatal(err_e1)
	}
	rr_e1 = httptest.NewRecorder()
	handler_e1 = http.HandlerFunc(hbRcv)
	handler_e1.ServeHTTP(rr_e1, req_e1)

	// Check the return code
	if rr_e1.Code != http.StatusBadRequest {
		t.Errorf("HTTP handler returned bad error code, got %v, want %v",
			rr_e1.Code, http.StatusBadRequest)
	}

	//Send a HB to an existing key

	req_e1_data = bytes.NewBufferString(`{"Component":"x1c2s2b0n3","Hostname":"nid0001.us.cray.com","NID":"0001","Status":"OK","Timestamp":"Jan 1, 0000"}`)
	req_e1, err_e1 = http.NewRequest("POST", "http://localhost:8080/hmi/v1/heartbeat", req_e1_data)

	if err_e1 != nil {
		t.Fatal(err_e1)
	}
	rr_e1 = httptest.NewRecorder()
	handler_e1 = http.HandlerFunc(hbRcv)
	handler_e1.ServeHTTP(rr_e1, req_e1)

	// Check the return code
	if rr_e1.Code != http.StatusOK {
		t.Errorf("HTTP handler returned bad error code, got %v, want %v",
			rr_e1.Code, http.StatusOK)
	}

	time.Sleep(1 * time.Second)
	t.Logf("  ==> FINISHED HEARTBEAT HTTP OPERATIONS TEST\n")
}

// Test entry point for params_io(), which is the params HTTP request handler.
// We will fake out an HTTP request object and record the response.
// We will also examine the current parameters to see if the PATCH operations
// work.
//
// t(in)   Test framework.
// Return: None.

func TestParams_io(t *testing.T) {
	t.Logf("** RUNNING PARAMETER HTTP OPERATIONS TEST\n")

	//Set parameters to initial values

	app_params.debug_level.int_param = 0
	app_params.warntime.int_param = 1
	app_params.errtime.int_param = 2

	//Slop together JSON payloads for 2 heartbeating nodes

	req1_hb := bytes.NewBufferString(`{"debug":"3","warntime":"5","errtime":"10"}`)

	// Create a fake HTTP PATCH with the param data in it
	req1, err1 := http.NewRequest("PATCH", "http://localhost:8080/hmi/v1/params", req1_hb)

	if err1 != nil {
		t.Fatal(err1)
	}

	// Set up to grab the "responses"
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(paramsIO)

	// Mock up the operation
	handler.ServeHTTP(rr, req1)

	// Check the return code
	if rr.Code != http.StatusOK {
		t.Errorf("HTTP handler returned bad error code, bot %v, want %v",
			rr.Code, http.StatusOK)
	}

	//Now check the current parameters to make sure they match

	if app_params.debug_level.int_param != 3 {
		t.Errorf("PATCH test failed: debug level is incorrect (exp: %d, got %d)",
			3, app_params.debug_level.int_param)
	}
	if app_params.warntime.int_param != 5 {
		t.Errorf("PATCH test failed: warntime is incorrect (exp: %d, got %d)",
			5, app_params.warntime.int_param)
	}
	if app_params.errtime.int_param != 10 {
		t.Errorf("PATCH test failed: errtime is incorrect (exp: %d, got %d)",
			10, app_params.errtime.int_param)
	}

	//Now do a GET operation and insure we get the same thing.

	req2, err2 := http.NewRequest("GET", "http://localhost:8080/hmi/v1/params", nil)

	if err2 != nil {
		t.Fatal(err2)
	}

	// Set up to grab the "responses"
	rr2 := httptest.NewRecorder()
	handler2 := http.HandlerFunc(paramsIO)

	// Mock up the operation
	handler2.ServeHTTP(rr2, req2)

	// Check the return code
	if rr2.Code != http.StatusOK {
		t.Errorf("HTTP handler returned bad error code, bot %v, want %v",
			rr2.Code, http.StatusOK)
	}

	// Read the response payload
	var pdata inidata
	body, err := ioutil.ReadAll(rr2.Body)
	if err != nil {
		t.Error("Error reading GET response data:", err)
	} else {
		errj := json.Unmarshal(body, &pdata)
		if errj != nil {
			t.Error("Error unmarshalling GET response data:", errj)
		} else {
			// Do the compares
			if pdata.Debug != "3" {
				t.Errorf("GET test failed: debug level is incorrect (exp: %d, got %s)",
					3, pdata.Debug)
			}
			if pdata.Warntime != "5" {
				t.Errorf("GET test failed: warntime is incorrect (exp: %d, got %s)",
					5, pdata.Warntime)
			}
			if pdata.Errtime != "10" {
				t.Errorf("GET test failed: errtime is incorrect (exp: %d, got %s)",
					10, pdata.Errtime)
			}
		}
	}

	//Now check some error conditions.  First, an invalid request (POST)

	req_e1_data := bytes.NewBufferString(`{"Debug":"3"}`)
	req_e1, err_e1 := http.NewRequest("POST", "http://localhost:8080/hmi/v1/params", nil)

	if err_e1 != nil {
		t.Fatal(err_e1)
	}
	rr_e1 := httptest.NewRecorder()
	handler_e1 := http.HandlerFunc(paramsIO)
	handler_e1.ServeHTTP(rr_e1, req_e1)

	// Check the return code
	if rr_e1.Code != http.StatusMethodNotAllowed {
		t.Errorf("HTTP handler returned bad error code, got %v, want %v",
			rr_e1.Code, http.StatusMethodNotAllowed)
	}

	//Check PATCH with bad data

	req_e1_data = bytes.NewBufferString(`{"Debug":3}`)
	req_e1, err_e1 = http.NewRequest("PATCH", "http://localhost:8080/hmi/v1/params", req_e1_data)

	if err_e1 != nil {
		t.Fatal(err_e1)
	}
	rr_e1 = httptest.NewRecorder()
	handler_e1 = http.HandlerFunc(paramsIO)
	handler_e1.ServeHTTP(rr_e1, req_e1)

	// Check the return code
	if rr_e1.Code != http.StatusBadRequest {
		t.Errorf("HTTP handler returned bad error code, got %v, want %v",
			rr_e1.Code, http.StatusBadRequest)
	}

	t.Logf("  ==> FINISHED PARAMETER HTTP OPERATIONS TEST\n")
}

func setSMRVal(val int) {
	SMRMutex.Lock()
	SMRval = val
	SMRMutex.Unlock()
}

func getSMRVal() int {
	var val int
	SMRMutex.Lock()
	val = SMRval
	SMRMutex.Unlock()
	return val
}

var gotUAHdr bool

func hasUserAgentHeader(r *http.Request) bool {
	if len(r.Header) == 0 {
		return false
	}

	_, ok := r.Header["User-Agent"]
	if !ok {
		return false
	}
	return true
}

func fakeHSMHandler(w http.ResponseWriter, req *http.Request) {
	gotUAHdr = hasUserAgentHeader(req)
	w.WriteHeader(getSMRVal())
}

var startComps, restartComps, stopWarnComps, stopErrorComps []string
var compLock sync.Mutex

func fakeHSMPatchHandler(w http.ResponseWriter, req *http.Request) {
	var sinfo smjbulk_v1
	if getSMRVal() != http.StatusOK {
		w.WriteHeader(getSMRVal())
		return
	}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = json.Unmarshal(body, &sinfo)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	compLock.Lock()
	if (sinfo.State == base.StateReady.String()) && (sinfo.Flag == base.FlagOK.String()) {
		if strings.Contains(sinfo.ExtendedInfo.Message, "beat restarted") {
			restartComps = append(restartComps, sinfo.ComponentIDs...)
		} else {
			startComps = append(startComps, sinfo.ComponentIDs...)
		}
	} else if (sinfo.State == base.StateReady.String()) && (sinfo.Flag == base.FlagWarning.String()) {
		stopWarnComps = append(stopWarnComps, sinfo.ComponentIDs...)
	} else if (sinfo.State == base.StateStandby.String()) && (sinfo.Flag == base.FlagAlert.String()) {
		stopErrorComps = append(stopErrorComps, sinfo.ComponentIDs...)
	}
	compLock.Unlock()

	w.WriteHeader(http.StatusOK)
}

func TestSMPatch1(t *testing.T) {
	htrans.transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	htrans.client = &http.Client{Transport: htrans.transport,
		Timeout: (time.Duration(app_params.statemgr_timeout.int_param) *
			time.Second),
	}
	smjinfo := smjbulk_v1{ComponentIDs: []string{"x0c0s0b0n0"}, State: "Ready", Flag: "OK",
		ExtendedInfo: smjson_einfo{Message: "Test"}}

	srv := httptest.NewServer(http.HandlerFunc(fakeHSMHandler))
	serviceName = "HBTDTest"

	app_params.statemgr_url.string_param = srv.URL
	app_params.nosm.int_param = 0
	hsmReady = true
	testMode = true

	//Run with a OK return from HSM

	t.Log("Running happy-path send_sm_patch test")
	gotUAHdr = false
	setSMRVal(http.StatusOK)
	hsmWG.Add(1)
	smjinfo.sentOK = false
	go send_sm_patch(&smjinfo)
	hsmWG.Wait()
	if !smjinfo.sentOK {
		t.Errorf("ERROR: send_sm_patch() didn't set sentOK flag.")
	}
	if !gotUAHdr {
		t.Errorf("ERROR, never saw User-Agent header.")
	}

	//Run with badness return (500)  Should be an error.

	t.Log("Running 500-path send_sm_patch test")
	setSMRVal(http.StatusInternalServerError)
	hsmWG.Add(1)
	smjinfo.sentOK = false
	go send_sm_patch(&smjinfo)
	hsmWG.Wait()
	if smjinfo.sentOK {
		t.Errorf("ERROR: send_sm_patch() incorrectly set sentOK flag.")
	}
}

func Test_HbStates(t *testing.T) {
	var kval, xname string
	var jdata hbStatesReq
	var rdata hbStatesRsp
	var err error

	t.Logf("** RUNNING hbStates TEST **")

	ots_err := one_time_setup()
	if ots_err != nil {
		t.Error("ERROR setting up KV store:", ots_err)
		return
	}
	staleKeys = false
	app_params.check_interval.int_param = 30
	app_params.warntime.int_param = 5
	app_params.errtime.int_param = 20

	//Make HB entries in KV store.  Need 3 -- one that's up to date, one that's
	//in warn state, one in err state.   Will query 4 nodes -- the above 3,
	//plus one that is not in the KV store.

	basetime := time.Now().Unix()
	xnameArr := []string{"x0c1s2b0n0", "x0c1s2b1n1", "x0c1s2b2n2", "x0c1s2b3n3"}
	times := []int64{0, 10, 30, 0}

	for ix := 0; ix < 3; ix++ {
		make_key(&kval, xnameArr[ix], (basetime - times[ix]))
		err = kvHandle.Store(xnameArr[ix], kval)
		if err != nil {
			t.Errorf("ERROR storing key data for '%s': %v", xnameArr[ix], err)
		}
	}

	//Do query for 4 nodes

	jdata.XNames = xnameArr
	ba, baerr := json.Marshal(&jdata)
	if baerr != nil {
		t.Error("ERROR marshalling POST data:", baerr)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(hbStates)
	req, _ := http.NewRequest("POST", "http://localhost:8080/hmi/v1/hbstates", bytes.NewBuffer(ba))

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HTTP handler returned bad status code: %v", rr.Code)
	}

	//Check results.

	body, bodyErr := ioutil.ReadAll(rr.Body)
	if bodyErr != nil {
		t.Errorf("ERROR reading hbstates return body: %v", bodyErr)
	}

	baerr = json.Unmarshal(body, &rdata)
	if baerr != nil {
		t.Errorf("ERROR umnarshalling hbstates return body: %v", baerr)
	}

	if len(rdata.HBStates) != 4 {
		t.Errorf("Invalid HB states length, expected 4, got %d", len(rdata.HBStates))
	}

	//Check results

	smap := make(map[string]*hbSingleStateRsp)

	for ix := 0; ix < len(rdata.HBStates); ix++ {
		smap[rdata.HBStates[ix].XName] = &rdata.HBStates[ix]
	}

	xname = xnameArr[0] //should be valid and heartbeating
	if smap[xname].Heartbeating == false {
		t.Errorf("ERROR, HB state for '%s' is not heartbeating, should be.", xname)
	}

	xname = xnameArr[1] //should be valid and heartbeating
	if smap[xname].Heartbeating == false {
		t.Errorf("ERROR, HB state for '%s' is not heartbeating, should be.", xname)
	}

	xname = xnameArr[2] //should be valid and not heartbeating
	if smap[xname].Heartbeating == true {
		t.Errorf("ERROR, HB state for '%s' is heartbeating, should not be.", xname)
	}

	xname = xnameArr[3] //should be not valid or heartbeating
	if smap[xname].Heartbeating == true {
		t.Errorf("ERROR, HB state for '%s' is heartbeating, should not be.", xname)
	}
}

func Test_HbStateSingle(t *testing.T) {
	var kval string
	var rdata hbSingleStateRsp
	var err error

	t.Logf("** RUNNING hbStates TEST **")

	ots_err := one_time_setup()
	if ots_err != nil {
		t.Error("ERROR setting up KV store:", ots_err)
		return
	}
	staleKeys = false
	app_params.check_interval.int_param = 30
	app_params.warntime.int_param = 5
	app_params.errtime.int_param = 20

	//Make HB entries in KV store.  Need 3 -- one that's up to date, one that's
	//in warn state, one in err state.   Will query 4 nodes -- the above 3,
	//plus one that is not in the KV store.

	basetime := time.Now().Unix()
	xnameArr := []string{"x1c1s2b0n0", "x1c1s2b1n1", "x1c1s2b2n2", "x1c1s2b3n3"}
	times := []int64{0, 10, 30, 0}

	for ix := 0; ix < 3; ix++ {
		make_key(&kval, xnameArr[ix], (basetime - times[ix]))
		err = kvHandle.Store(xnameArr[ix], kval)
		if err != nil {
			t.Errorf("ERROR storing key data for '%s': %v", xnameArr[ix], err)
		}
	}

	//Do queries for 4 nodes.  Note that we MUST use the mux since we're
	//utilizing the {xname} portion of the URL.

	routes := generateRoutes()
	router = newRouter(routes)

	//First: valid and heartbeating

	rr := httptest.NewRecorder()
	url := fmt.Sprintf("http://localhost/hmi/v1/hbstate/%s", xnameArr[0])
	req, _ := http.NewRequest("GET", url, nil)

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HTTP handler returned bad status code: %v", rr.Code)
	}

	//Check results.

	body, bodyErr := ioutil.ReadAll(rr.Body)
	if bodyErr != nil {
		t.Errorf("ERROR reading hbstates return body: %v", bodyErr)
	}

	err = json.Unmarshal(body, &rdata)
	if err != nil {
		t.Errorf("ERROR umnarshalling hbstates return body: %v", err)
	}

	//Should be valid and heartbeating
	if rdata.XName != xnameArr[0] {
		t.Errorf("ERROR, HB state for '%s' has invalid xname: '%s'",
			xnameArr[0], rdata.XName)
	}
	if rdata.Heartbeating == false {
		t.Errorf("ERROR, HB state for '%s' is not heartbeating, should be.",
			rdata.XName)
	}

	//Second: Should be valid and heartbeating

	rr = httptest.NewRecorder()
	url = fmt.Sprintf("http://localhost/hmi/v1/hbstate/%s", xnameArr[1])
	req, _ = http.NewRequest("GET", url, nil)

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HTTP handler returned bad status code: %v", rr.Code)
	}

	//Check results.

	body, bodyErr = ioutil.ReadAll(rr.Body)
	if bodyErr != nil {
		t.Errorf("ERROR reading hbstates return body: %v", bodyErr)
	}

	err = json.Unmarshal(body, &rdata)
	if err != nil {
		t.Errorf("ERROR umnarshalling hbstates return body: %v", err)
	}

	//Should be valid and heartbeating
	if rdata.XName != xnameArr[1] {
		t.Errorf("ERROR, HB state for '%s' has invalid xname: '%s'",
			xnameArr[1], rdata.XName)
	}
	if rdata.Heartbeating == false {
		t.Errorf("ERROR, HB state for '%s' is not heartbeating, should be.",
			rdata.XName)
	}

	//Third: should be valid, not heartbeating

	rr = httptest.NewRecorder()
	url = fmt.Sprintf("http://localhost/hmi/v1/hbstate/%s", xnameArr[2])
	req, _ = http.NewRequest("GET", url, nil)

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HTTP handler returned bad status code: %v", rr.Code)
	}

	//Check results.

	body, bodyErr = ioutil.ReadAll(rr.Body)
	if bodyErr != nil {
		t.Errorf("ERROR reading hbstates return body: %v", bodyErr)
	}

	err = json.Unmarshal(body, &rdata)
	if err != nil {
		t.Errorf("ERROR umnarshalling hbstates return body: %v", err)
	}

	//Should be valid and NOT heartbeating
	if rdata.XName != xnameArr[2] {
		t.Errorf("ERROR, HB state for '%s' has invalid xname: '%s'",
			xnameArr[2], rdata.XName)
	}
	if rdata.Heartbeating == true {
		t.Errorf("ERROR, HB state for '%s' is heartbeating, should not be.",
			rdata.XName)
	}

	//Fourth: not valid

	rr = httptest.NewRecorder()
	url = fmt.Sprintf("http://localhost/hmi/v1/hbstate/%s", xnameArr[3])
	req, _ = http.NewRequest("GET", url, nil)

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("HTTP handler returned bad status code: %v", rr.Code)
	}

	//Check results.

	body, bodyErr = ioutil.ReadAll(rr.Body)
	if bodyErr != nil {
		t.Errorf("ERROR reading hbstates return body: %v", bodyErr)
	}

	err = json.Unmarshal(body, &rdata)
	if err != nil {
		t.Errorf("ERROR umnarshalling hbstates return body: %v", err)
	}

	//Should be invalid, not heartbeating
	if rdata.XName != xnameArr[3] {
		t.Errorf("ERROR, HB state for '%s' has invalid xname: '%s'",
			xnameArr[3], rdata.XName)
	}
	if rdata.Heartbeating == true {
		t.Errorf("ERROR, HB state for '%s' is heartbeating, should not be.",
			rdata.XName)
	}
}

func TestHBMapping(t *testing.T) {
	hbi1 := hbinfo{Component: "x0c0s0b0n1", Last_hb_rcv_time: "00000001",
		Last_hb_timestamp: "00000001", Last_hb_status: "OK"}
	hbi2 := hbinfo{Component: "x0c0s0b0n2", Last_hb_rcv_time: "00000002",
		Last_hb_timestamp: "00000002", Last_hb_status: "OK"}
	hbi3 := hbinfo{Component: "x0c0s0b0n3", Last_hb_rcv_time: "00000003",
		Last_hb_timestamp: "00000003", Last_hb_status: "OK"}
	hbi4 := hbinfo{Component: "x0c0s0b0n4", Last_hb_rcv_time: "00000004",
		Last_hb_timestamp: "00000004", Last_hb_status: "OK"}

	kill_sm_goroutines()
	go send_sm_req()

	htrans.transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	htrans.client = &http.Client{Transport: htrans.transport,
		Timeout: (20 * time.Second),
	}
	srv := httptest.NewServer(http.HandlerFunc(fakeHSMPatchHandler))

	app_params.statemgr_url.string_param = srv.URL
	app_params.statemgr_timeout.int_param = 5
	app_params.nosm.int_param = 0
	testMode = true
	hsmReady = true

	compLock.Lock()
	hbMapLock.Lock()
	startComps = []string{}
	restartComps = []string{}
	stopWarnComps = []string{}
	stopErrorComps = []string{}
	for k, _ := range StartMap {
		StartMap[k] = 0
	}
	for k, _ := range RestartMap {
		RestartMap[k] = 0
	}
	for k, _ := range StopWarnMap {
		StopWarnMap[k] = 0
	}
	for k, _ := range StopErrorMap {
		StopErrorMap[k] = 0
	}
	compLock.Unlock()
	hbMapLock.Unlock()

	is_setup = 0
	one_time_setup()

	testMode = true

	hb_update_notify(&hbi1, HB_started)
	hb_update_notify(&hbi2, HB_restarted_warn)
	hb_update_notify(&hbi3, HB_stopped_warn)
	hb_update_notify(&hbi4, HB_stopped_error)

	//Conflicts
	hb_update_notify(&hbi1, HB_restarted_warn)
	hb_update_notify(&hbi2, HB_stopped_warn)
	hb_update_notify(&hbi3, HB_stopped_error)
	hb_update_notify(&hbi4, HB_started)

	setSMRVal(http.StatusOK)
	hsmUpdateQ <- HSMQ_NEW
	time.Sleep(2 * time.Second)

	//Check if we got the results we wanted.

	if len(startComps) == 0 {
		t.Fatalf("No 'start' components found.")
	}
	if len(restartComps) == 0 {
		t.Fatalf("No 'restart' components found.")
	}
	if len(stopWarnComps) == 0 {
		t.Fatalf("No 'stop-warn' components found.")
	}
	if len(stopErrorComps) == 0 {
		t.Fatalf("No 'stop-error' components found.")
	}

	if len(startComps) > 1 {
		t.Errorf("Too many 'start' components (%d), expected 1.", len(startComps))
	}
	if len(restartComps) > 1 {
		t.Errorf("Too many 'restart' components (%d), expected 1.", len(restartComps))
	}
	if len(stopWarnComps) > 1 {
		t.Errorf("Too many 'stopWarnComps' components (%d), expected 1.", len(stopWarnComps))
	}
	if len(stopErrorComps) > 1 {
		t.Errorf("Too many 'stopErrorComps' components (%d), expected 1.", len(stopErrorComps))
	}

	if startComps[0] != hbi4.Component {
		t.Errorf("HB start not found on '%s'", hbi4.Component)
	}
	if restartComps[0] != hbi1.Component {
		t.Errorf("HB restart not found on '%s'", hbi1.Component)
	}
	if stopWarnComps[0] != hbi2.Component {
		t.Errorf("HB stop-warning not found on '%s'", hbi2.Component)
	}
	if stopErrorComps[0] != hbi3.Component {
		t.Errorf("HB stop-error not found on '%s'", hbi3.Component)
	}

	//Make sure global maps are cleared

	nitems := 0
	for _, v := range StartMap {
		if v != 0 {
			nitems++
		}
	}
	if nitems > 0 {
		t.Errorf("Global Start Map is not cleared.")
	}
	nitems = 0
	for _, v := range RestartMap {
		if v != 0 {
			nitems++
		}
	}
	if nitems > 0 {
		t.Errorf("Global Restart Map is not cleared.")
	}
	nitems = 0
	for _, v := range StopWarnMap {
		if v != 0 {
			nitems++
		}
	}
	if nitems > 0 {
		t.Errorf("Global Stop Warning Map is not cleared.")
	}
	nitems = 0
	for _, v := range StopErrorMap {
		if v != 0 {
			nitems++
		}
	}
	if nitems > 0 {
		t.Errorf("Global Stop Error Map is not cleared.")
	}
}

func TestHBMapping2(t *testing.T) {
	hbi1 := hbinfo{Component: "x0c0s0b0n0", Last_hb_rcv_time: "00000000",
		Last_hb_timestamp: "00000000", Last_hb_status: "OK"}
	hbi2 := hbinfo{Component: "x0c0s0b0n1", Last_hb_rcv_time: "00000001",
		Last_hb_timestamp: "00000001", Last_hb_status: "OK"}
	hbi3 := hbinfo{Component: "x0c0s0b0n2", Last_hb_rcv_time: "00000002",
		Last_hb_timestamp: "00000002", Last_hb_status: "OK"}
	hbi4 := hbinfo{Component: "x0c0s0b0n3", Last_hb_rcv_time: "00000003",
		Last_hb_timestamp: "00000003", Last_hb_status: "OK"}
	hbi5 := hbinfo{Component: "x0c0s0b0n4", Last_hb_rcv_time: "00000004",
		Last_hb_timestamp: "00000004", Last_hb_status: "OK"}

	kill_sm_goroutines()
	go send_sm_req()

	htrans.transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	htrans.client = &http.Client{Transport: htrans.transport,
		Timeout: (20 * time.Second),
	}
	srv := httptest.NewServer(http.HandlerFunc(fakeHSMPatchHandler))

	app_params.statemgr_url.string_param = srv.URL
	app_params.statemgr_timeout.int_param = 5
	app_params.nosm.int_param = 0
	testMode = true
	hsmReady = true

	compLock.Lock()
	startComps = []string{}
	restartComps = []string{}
	stopWarnComps = []string{}
	stopErrorComps = []string{}
	compLock.Unlock()

	hb_update_notify(&hbi1, HB_started)
	hb_update_notify(&hbi2, HB_restarted_warn)
	hb_update_notify(&hbi3, HB_stopped_warn)
	hb_update_notify(&hbi4, HB_stopped_error)

	hb_update_notify(&hbi1, HB_restarted_warn)
	hb_update_notify(&hbi2, HB_stopped_warn)
	hb_update_notify(&hbi3, HB_stopped_error)
	hb_update_notify(&hbi4, HB_started)

	setSMRVal(http.StatusNotFound)
	hsmUpdateQ <- HSMQ_NEW
	time.Sleep(2 * time.Second)

	//Check if we got the results we wanted.

	if len(startComps) != 0 {
		t.Fatalf("'start' components incorrectly found.")
	}
	if len(restartComps) != 0 {
		t.Fatalf("No 'restart' components incorrectly found.")
	}
	if len(stopWarnComps) != 0 {
		t.Fatalf("No 'stop-warn' components incorrectly found.")
	}
	if len(stopErrorComps) != 0 {
		t.Fatalf("No 'stop-error' components incorrectly found.")
	}

	//Make sure global maps are NOT cleared

	nitems := 0
	for _, v := range StartMap {
		if v != 0 {
			nitems++
		}
	}
	if nitems != 0 {
		t.Errorf("Global Start Map is not cleared.")
	}
	nitems = 0
	for _, v := range RestartMap {
		if v != 0 {
			nitems++
		}
	}
	if nitems != 0 {
		t.Errorf("Global Restart Map is not cleared.")
	}
	nitems = 0
	for _, v := range StopWarnMap {
		if v != 0 {
			nitems++
		}
	}
	if nitems != 0 {
		t.Errorf("Global Stop Warning Map is not cleared.")
	}
	nitems = 0
	for _, v := range StopErrorMap {
		if v != 0 {
			nitems++
		}
	}
	if nitems != 0 {
		t.Errorf("Global Stop Error Map is not cleared.")
	}

	//Now do it again making sure we get the "superset"

	compLock.Lock()
	startComps = []string{}
	restartComps = []string{}
	stopWarnComps = []string{}
	stopErrorComps = []string{}
	compLock.Unlock()

	hb_update_notify(&hbi5, HB_started)

	setSMRVal(http.StatusOK)
	hsmUpdateQ <- HSMQ_NEW
	time.Sleep(2 * time.Second)

	//Check if we got the results we wanted.

	if len(startComps) == 0 {
		t.Fatalf("No 'start' components found.")
	}
	if len(restartComps) == 0 {
		t.Fatalf("No 'restart' components found.")
	}
	if len(stopWarnComps) == 0 {
		t.Fatalf("No 'stop-warn' components found.")
	}
	if len(stopErrorComps) == 0 {
		t.Fatalf("No 'stop-error' components found.")
	}

	if len(startComps) > 2 {
		t.Errorf("Too many 'start' components (%d), expected 2.", len(startComps))
	}
	if len(restartComps) > 1 {
		t.Errorf("Too many 'restart' components (%d), expected 1.", len(restartComps))
	}
	if len(stopWarnComps) > 1 {
		t.Errorf("Too many 'stopWarnComps' components (%d), expected 1.", len(stopWarnComps))
	}
	if len(stopErrorComps) > 1 {
		t.Errorf("Too many 'stopErrorComps' components (%d), expected 1.", len(stopErrorComps))
	}

	if (startComps[0] != hbi4.Component) &&
		(startComps[1] != hbi4.Component) {
		t.Errorf("HB start not found on '%s'", hbi4.Component)
	}
	if (startComps[0] != hbi5.Component) &&
		(startComps[1] != hbi5.Component) {
		t.Errorf("HB start not found on '%s'", hbi5.Component)
	}
	if restartComps[0] != hbi1.Component {
		t.Errorf("HB restart not found on '%s'", hbi1.Component)
	}
	if stopWarnComps[0] != hbi2.Component {
		t.Errorf("HB stop-warning not found on '%s'", hbi2.Component)
	}
	if stopErrorComps[0] != hbi3.Component {
		t.Errorf("HB stop-error not found on '%s'", hbi3.Component)
	}
}
