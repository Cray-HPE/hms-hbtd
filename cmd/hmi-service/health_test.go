// MIT License
//
// (C) Copyright [2020-2021] Hewlett Packard Enterprise Development LP
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
	"fmt"
	"crypto/tls"
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"stash.us.cray.com/HMS/hms-hmetcd"
)

func TestLiveness(t *testing.T) {
	// intialize a bunch of stuff for the tests
	var ba []byte
	reqPayload := bytes.NewBuffer(ba)
	handler1 := http.HandlerFunc(doLiveness)

	// test valid request
	req1, _ := http.NewRequest("GET", "http://localhost:8080/hmi/v1/liveness", reqPayload)
	rr1 := httptest.NewRecorder()
	handler1.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusNoContent {
		t.Errorf("GET operation failed, got response code %v\n", rr1.Code)
	}

	// test invalid request
	req2, _ := http.NewRequest("PUT", "http://localhost:8080/hmi/v1/liveness", reqPayload)
	rr2 := httptest.NewRecorder()
	handler1.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT operation failed, got response code %v\n", rr2.Code)
	}

	// test invalid request
	req3, _ := http.NewRequest("POST", "http://localhost:8080/hmi/v1/liveness", reqPayload)
	rr3 := httptest.NewRecorder()
	handler1.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST operation failed, got response code %v\n", rr3.Code)
	}
}

func TestReadiness(t *testing.T) {
	// intialize a bunch of stuff for the tests
	var ba []byte
	reqPayload := bytes.NewBuffer(ba)
	handler1 := http.HandlerFunc(doReadiness)

	// test not ready request - KV Store not initialized
	req1, _ := http.NewRequest("GET", "http://localhost:8080/hmi/v1/readiness", reqPayload)
	rr1 := httptest.NewRecorder()
	handler1.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusServiceUnavailable {
		t.Errorf("1) GET operation failed, got response code %v\n", rr1.Code)
	}

	// test invalid request
	req2, _ := http.NewRequest("PUT", "http://localhost:8080/hmi/v1/readiness", reqPayload)
	rr2 := httptest.NewRecorder()
	handler1.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT operation failed, got response code %v\n", rr2.Code)
	}

	// test invalid request
	req3, _ := http.NewRequest("POST", "http://localhost:8080/hmi/v1/readiness", reqPayload)
	rr3 := httptest.NewRecorder()
	handler1.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST operation failed, got response code %v\n", rr3.Code)
	}

	// take existing KVStore if present and set aside - make sure
	// the existing one gets restored on exit
	pickledKV := kvHandle
	kvHandle = nil
	defer func() { kvHandle = pickledKV }()
	var kverr error
	kvHandle, kverr = hmetcd.Open("mem:", "")
	if kverr != nil {
		t.Fatal("KV/ETCD open failed:", kverr)
	}
	kvHandle.Store(HBTD_HEALTH_KEY, HBTD_HEALTH_OK)

	// now test that it is ready
	req4, _ := http.NewRequest("GET", "http://localhost:8080/hmi/v1/readiness", reqPayload)
	rr4 := httptest.NewRecorder()
	handler1.ServeHTTP(rr4, req4)
	if rr4.Code != http.StatusNoContent {
		t.Errorf("2) GET operation failed, got response code %v\n", rr1.Code)
	}
}

func TestHealth(t *testing.T) {
	// intialize a bunch of stuff for the tests
	handler1 := http.HandlerFunc(doHealth)

	// take existing KVStore if present and set aside - make sure
	// the existing one gets restored on exit
	pickledKV := kvHandle
	kvHandle = nil
	defer func() { kvHandle = pickledKV }()

	// test valid request
	req1, _ := http.NewRequest("GET", "http://localhost:8080/hmi/v1/health", nil)
	rr1 := httptest.NewRecorder()
	handler1.ServeHTTP(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("GET operation failed, got response code %v\n", rr1.Code)
	}
	body, err := ioutil.ReadAll(rr1.Body)
	if err != nil {
		t.Fatal("ERROR reading GET response body:", err)
	}
	var stats HealthResponse
	err = json.Unmarshal(body, &stats)
	if err != nil {
		t.Fatalf("ERROR unmarshalling GET response body:%s", err.Error())
	}
	if stats.KvStoreStatus != "Not initialized" {
		t.Fatal("Expected KV Store not initialized")
	}
	if stats.HsmStatus != "Not initialized" {
		t.Fatal("Expected HSM not initialized")
	}

	// test invalid request
	req2, _ := http.NewRequest("PUT", "http://localhost:8080/hmi/v1/health", nil)
	rr2 := httptest.NewRecorder()
	handler1.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT operation failed, got response code %v\n", rr2.Code)
	}

	// test invalid request
	req3, _ := http.NewRequest("POST", "http://localhost:8080/hmi/v1/health", nil)
	rr3 := httptest.NewRecorder()
	handler1.ServeHTTP(rr3, req3)
	if rr3.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST operation failed, got response code %v\n", rr3.Code)
	}

	// start the KV Store and test again
	var kverr error
	kvHandle, kverr = hmetcd.Open("mem:", "")
	if kverr != nil {
		t.Fatal("KV/ETCD open failed:", kverr)
	}

	//First test with no health key present

	kvHandle.Delete(HBTD_HEALTH_KEY)
	rr5 := httptest.NewRecorder()
	handler1.ServeHTTP(rr5, req1)
	if rr5.Code != http.StatusOK {
		t.Errorf("GET operation failed, got response code %v\n", rr5.Code)
	}
	body5, err5 := ioutil.ReadAll(rr5.Body)
	if err5 != nil {
		t.Fatal("ERROR reading GET response body:", err5)
	}
	err = json.Unmarshal(body5, &stats)
	if err != nil {
		t.Fatalf("ERROR unmarshalling GET response body:%s", err.Error())
	}
	kp := fmt.Sprintf("Initialization key not present")
	if stats.KvStoreStatus != kp {
		t.Fatal("Expected KV Store to be un-initialized")
	}

	kvHandle.Store(HBTD_HEALTH_KEY, HBTD_HEALTH_OK)

	// now test with KV Store present
	rr4 := httptest.NewRecorder()
	handler1.ServeHTTP(rr4, req1)
	if rr1.Code != http.StatusOK {
		t.Errorf("GET operation failed, got response code %v\n", rr4.Code)
	}
	body4, err := ioutil.ReadAll(rr4.Body)
	if err != nil {
		t.Fatal("ERROR reading GET response body:", err)
	}
	err = json.Unmarshal(body4, &stats)
	if err != nil {
		t.Fatalf("ERROR unmarshalling GET response body:%s", err.Error())
	}
	kp = fmt.Sprintf("Initialization key present: %s",HBTD_HEALTH_OK)
	if stats.KvStoreStatus != kp {
		t.Fatal("Expected KV Store initialized")
	}
	if stats.HsmStatus != "Not initialized" {
		t.Fatal("Expected HSM not initialized")
	}
}

func TestHSMReadies(t *testing.T) {
	// Set up transport

	htrans.transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	htrans.client = &http.Client{Transport: htrans.transport,
		Timeout: (time.Duration(app_params.statemgr_timeout.int_param) *
			time.Second),
	}

	t.Logf("**** Testing HSM readiness ****")
	go checkHSM()
	time.Sleep(10 * time.Second)
	if (hsmReady == true) {
		t.Errorf("HSM shows ready, should not be.")
	}
	stopCheckHSM = true
	time.Sleep(8 * time.Second)
	stopCheckHSM = false

	go func() {
		time.Sleep(5 * time.Second)
		hsmReady = true
		time.Sleep(5 * time.Second)
	}()

	t.Logf("*** Waiting for HSM ready ***")
	waitForHSM()
	t.Logf("*** Done Waiting for HSM ready ***")
	hsmReady = false
}
