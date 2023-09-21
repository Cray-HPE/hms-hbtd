// MIT License
//
// (C) Copyright [2020-2021,2023] Hewlett Packard Enterprise Development LP
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
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Cray-HPE/hms-base"
)

// HealthResponse - used to report service health stats
type HealthResponse struct {
	KvStoreStatus string `json:"KvStore"`
	MsgBusStatus  string `json:"MsgBus"`
	HsmStatus     string `json:"HsmStatus"`
}

var hsmReady = false
var stopCheckHSM = false

// Periodically check on the availability of HSM.

func checkHSM() {
	var offBase int64

	if app_params.nosm.int_param != 0 {
		return
	}

	url := app_params.statemgr_url.string_param + "/" + SM_URL_READY
	pstat := false
	offBase = time.Now().Unix()

	for {
		if stopCheckHSM {
			return
		}
		lrdy := false
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			hbtdPrintf("ERROR: HSM check, can't create request: %v", err)
		} else {
			base.SetHTTPUserAgent(req, serviceName)
			rsp, rerr := htrans.client.Do(req)
			if rerr == nil {
				if rsp.Body != nil {
					rsp.Body.Close()
				}
				if rsp.StatusCode == http.StatusOK {
					lrdy = true
				}
			}
		}

		hsmReady = lrdy
		if !lrdy {
			tdiff := time.Now().Unix() - offBase
			hbtdPrintf("HSM is not responsive (%d seconds).", tdiff)
		} else {
			offBase = time.Now().Unix()
			if !pstat {
				offBase = time.Now().Unix()
				hbtdPrintf("HSM is responsive.")
			}
		}
		pstat = lrdy
		time.Sleep(5 * time.Second)
	}
}

//Wait until HSM is ready.  NOTE: this may be temporary.  There needs to
//be improvements on how to handle HSM coming and going.  Once that is
//done this may be able to be removed.

func waitForHSM() {
	if app_params.nosm.int_param != 0 {
		return
	}

	hbtdPrintf("Waiting for HSM to be responsive...")
	for {
		if hsmReady {
			return
		}

		time.Sleep(3 * time.Second)
	}
}

// doHealth - returns useful information about the service to the user
func doHealth(w http.ResponseWriter, r *http.Request) {
	// NOTE: this is provided as a debugging aid for administrators to
	//  find out what is going on with the system.  This should return
	//  information in a human-readable format that will help to
	//  determine the state of this service.

	// only allow 'GET' calls
	errinst := "/" + URL_HEALTH
	if r.Method != http.MethodGet {
		log.Printf("ERROR: request is not a GET.\n")
		pdet := base.NewProblemDetails("about:blank",
			"Invalid Request",
			"Only GET operation supported",
			errinst, http.StatusMethodNotAllowed)
		//It is required to have an "Allow:" header with this error
		w.Header().Add("Allow", "GET")
		base.SendProblemDetails(w, pdet, 0)
		return
	}

	// collect health information
	var stats HealthResponse

	// KV Store: openKV()
	if kvHandle == nil {
		// handle not created yet
		stats.KvStoreStatus = "Not initialized"
	} else {
		// handle there, lets see what we can do
		kvVal, kvOk, kerr := kvHandle.Get(HBTD_HEALTH_KEY)
		if kerr != nil {
			// report the error
			stats.KvStoreStatus = fmt.Sprintf("Error accessing key values:%s", kerr.Error())
		} else if !kvOk {
			// can access kv store, but initialization string not there
			// NOTE: looks like same one that hmnfd uses - that a problem???
			stats.KvStoreStatus = "Initialization key not present"
		} else {
			stats.KvStoreStatus = fmt.Sprintf("Initialization key present: %s", kvVal)
		}
	}

	// Send an 'are you ready' request to hardware state manager
	if htrans.client == nil {
		stats.HsmStatus = fmt.Sprintf("Not initialized")
	} else if !hsmReady {
		stats.HsmStatus = "Not ready"
	} else {
		stats.HsmStatus = "Ready"
	}

	// Telemetry bus
	if msgbusHandle != nil {
		// NOTE: status==1 -> Open, 2 -> closed (msgbus.go defs of StatusOpen, StatusClosed)
		st := msgbusHandle.Status()
		if st == 1 {
			stats.MsgBusStatus = "Connected and OPEN"
		} else if st == 2 {
			stats.MsgBusStatus = "Connected and CLOSED"
		} else {
			stats.MsgBusStatus = fmt.Sprintf("Connected with unknown status:%d", st)
		}
	} else {
		stats.MsgBusStatus = "Not Connected"
	}

	// write the output
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
	return
}

// doReadiness - used for k8s readiness check
func doReadiness(w http.ResponseWriter, r *http.Request) {
	// NOTE: this is coded in accordance with kubernetes best practices
	//  for liveness/readiness checks.  This function should only be
	//  used to indicate if something is wrong with this service that
	//  prevents usage.  If this fails too many times, the instance
	//  will be killed and re-started.  Only fail this if restarting
	//  this service is likely to fix the problem.

	// only allow 'GET' calls
	errinst := "/" + URL_READINESS
	if r.Method != http.MethodGet {
		log.Printf("ERROR: request is not a GET.\n")
		pdet := base.NewProblemDetails("about:blank",
			"Invalid Request",
			"Only GET operation supported",
			errinst, http.StatusMethodNotAllowed)
		//It is required to have an "Allow:" header with this error
		w.Header().Add("Allow", "GET")
		base.SendProblemDetails(w, pdet, 0)
		return
	}

	// check the readiness of dependencies that a restart may help with
	ready := true

	// Dependencies that a restart will probably not help with:
	// Kafka bus - will just keep trying to connect - need to restart if lost???
	// State manager - need it there, but a restart won't help

	// Critical dependencies that a restart may help
	// KV Store - had to be established before startup - CLBO if not originally connected
	if kvHandle == nil {
		// handle not created yet
		log.Printf("INFO: doReadiness check: KV Store not initialized")
		ready = false
	} else {
		// handle there, only interested if the call succeeds
		_, _, kerr := kvHandle.Get(HBTD_HEALTH_KEY)
		if kerr != nil {
			log.Printf("INFO: doReadiness check: KV Store error:%s", kerr.Error())
			ready = false
		}
	}

	// form the reply
	if ready {
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	return
}

// doLiveness - used for k8s liveness check
func doLiveness(w http.ResponseWriter, r *http.Request) {
	// NOTE: this is coded in accordance with kubernetes best practices
	//  for liveness/readiness checks.  This function should only be
	//  used to indicate the server is still alive and processing requests.

	// only allow 'GET' calls
	errinst := "/" + URL_LIVENESS
	if r.Method != http.MethodGet {
		log.Printf("ERROR: request is not a GET.\n")
		pdet := base.NewProblemDetails("about:blank",
			"Invalid Request",
			"Only GET operation supported",
			errinst, http.StatusMethodNotAllowed)
		//It is required to have an "Allow:" header with this error
		w.Header().Add("Allow", "GET")
		base.SendProblemDetails(w, pdet, 0)
		return
	}

	// return simple StatusOK response to indicate server is alive
	w.WriteHeader(http.StatusNoContent)
	return
}
