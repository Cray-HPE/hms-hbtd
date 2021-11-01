// MIT License
// 
// (C) Copyright [2018-2021] Hewlett Packard Enterprise Development LP
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
    "net/http"
    "encoding/json"
    "time"
    "io/ioutil"
    "bytes"
    "context"
    "strconv"
    "github.com/Cray-HPE/hms-base"
    "reflect"
    "fmt"
    "os"
    "strings"
    "sync"

    "github.com/gorilla/mux"
    "github.com/Cray-HPE/hms-hmetcd"
)

/////////////////////////////////////////////////////////////////////////////
// Data structures
/////////////////////////////////////////////////////////////////////////////

// Data structure to hold heartbeat tracking info

type hbinfo struct {
    Component string         `json:"Component"`         //Component XName
    Last_hb_rcv_time string  `json:"Last_hb_rcv_time"`  //Time last HB was received
    Last_hb_timestamp string `json:"Last_hb_timestamp"` //ISO8601 time stamp, set by sender
    Last_hb_status string    `json:"Last_hb_status"`    //Any special status of last HB, from sender
    Had_warning string       `json:"Had_warning"`       //Flag to mark start/stop edge conditions
}

// Heartbeat JSON.  This is the HB message format, which must follow all
// versioning constraints.

type hbjson_full_v1 struct {
    Component string `json:"Component"`
    Hostname string  `json:"Hostname"`
    NID string       `json:"NID"`
    Status string    `json:"Status"`
    Timestamp string `json:"Timestamp"`
}

type hbjson_v1 struct {
    Status string    `json:"Status"`
    Timestamp string `json:"Timestamp"`
}

// Data passed to the SM message sender thread

type sminfo struct {
    component string
    status string
    to_state int
    last_hb_timestamp string
}

// For sending PATCH to /State/Components/{xname}/StateData

type smjson_einfo struct {
    Id string      `json:"ID"`
    Message string `json:"Message"`
    Flag string    `json:"Flag"`
}

type smjbulk_v1 struct {
    ComponentIDs []string `json:"ComponentIDs"`
    State        string   `json:"State"`
    Flag         string   `json:"Flag"`
    ExtendedInfo smjson_einfo

	//Unmarshallable, used for HSM communication status
    needSend     bool
    sentOK       bool
}

// for sending HB state changes to the telemetry bus

type telemetry_json_v1 struct {
    MessageID string       `json:"MessageID"`
    Id string              `json:"ID"`
    NewState string        `json:"NewState"`
    NewFlag string         `json:"NewFlag"`
    LastHBTimeStamp string `json:"LastHBTimeStamp"`
    Info string            `json:"Info"`
}

type hbSingleStateRsp struct {
	XName string      `json:"XName"`
	Heartbeating bool `json:"Heartbeating"`
}

type hbStatesRsp struct {
	HBStates []hbSingleStateRsp `json:"HBStates"`
}

type hbStatesReq struct {
	XNames []string `json:"XNames"`
}

/////////////////////////////////////////////////////////////////////////////
// Constants and enums
/////////////////////////////////////////////////////////////////////////////

// HB states

const (
    HB_started        = 1
    HB_stopped_warn   = 2
    HB_restarted_warn = 3
    HB_stopped_error  = 4
    HB_quit           = 0x8675309
)

const (
    HB_WARN_NONE   = ""
    HB_WARN_NORMAL = "WN"
    HB_WARN_GAP    = "WG"
)

const TELEMETRY_MESSAGE_ID = "Heartbeat Change Notification"

// Values used to signify HSM processing activity

const HSMQ_DIE = 0x8675309
const HSMQ_NEW = 0x11223344


/////////////////////////////////////////////////////////////////////////////
// Global variables
/////////////////////////////////////////////////////////////////////////////

// Chan/async Qs

var hsmUpdateQ   = make(chan int, 100000)
var telemetryQ   = make(chan telemetry_json_v1, 50000)
var StartMap     = make(map[string]uint64)
var RestartMap   = make(map[string]uint64)
var StopWarnMap  = make(map[string]uint64)
var StopErrorMap = make(map[string]uint64)
var hbSeq uint64
var hsmWG sync.WaitGroup
var hbMapLock sync.Mutex
var testMode bool

// Used to track the number of components currently tracked

var sg_ncomp = 0


/////////////////////////////////////////////////////////////////////////////
// Send a patch to the State Mgr to indicate heartbeat status change.  Note 
// that this is called as part of a wait group for synchronization.  The 
// result of the operation (pass/fail) is indicated in the passed-in data
// struct.
//
// smjinfo(in): Bulk state update data to send to HSM.
// Return:      None.
/////////////////////////////////////////////////////////////////////////////

func send_sm_patch(smjinfo *smjbulk_v1) {
    defer hsmWG.Done()

    barr,err := json.Marshal(smjinfo)
    if (err != nil) {
        hbtdPrintln("INTERNAL ERROR marshalling SM info:",err)
        return
    }

    url := app_params.statemgr_url.string_param

    if (!testMode) {
        url = url + "/" + SM_URL_MID + "/" + SM_URL_SUFFIX
    }

    if (app_params.debug_level.int_param > 1) {
        hbtdPrintf("Sending PATCH to State Mgr URL: '%s', Data: '%s'",
            url,string(barr))
    }

    //Don't actually send anything to the SM if we're in "--nosm" mode.

    if (app_params.nosm.int_param != 0) {
        return
    }

    // Make PATCH requests this way since http.Client has no Patch() method.

    ctx,cancel := context.WithTimeout(context.Background(),
	                  (time.Duration(app_params.statemgr_timeout.int_param) *
	                   time.Second))
    defer cancel()
    req,_ := http.NewRequestWithContext(ctx,"PATCH", url, bytes.NewBuffer(barr))
    req.Header.Set("Content-Type","application/json")
    base.SetHTTPUserAgent(req,serviceName)

    rsp,err := htrans.client.Do(req)

    if (err != nil) {
        hbtdPrintln("ERROR sending PATCH to SM:",err)
        return
    } else {
        defer rsp.Body.Close()
        _,_ = ioutil.ReadAll(rsp.Body)
        if ((rsp.StatusCode == http.StatusOK) ||
            (rsp.StatusCode == http.StatusNoContent) ||
            (rsp.StatusCode == http.StatusAccepted)) {
            if (app_params.debug_level.int_param > 1) {
                hbtdPrintln("SUCCESS sending PATCH to SM, response:",rsp)
            }
        } else {
            hbtdPrintln("ERROR response from State Manager:",rsp.Status,"Error code:",rsp.StatusCode)
            return
        }
    }

    smjinfo.sentOK = true
    return
}

// Convenience function.  Takes component heartbeat status maps and prepares
// them for HB change processing.  Populates an "all components map" that is
// the superset of recent HB changes to persistent ones.

func groomCompLocalMapsPRE(allCompsMap map[string]bool, cpStartMap,cpRestartMap,cpStopWarnMap,cpStopErrorMap map[string]uint64) {
	//For each HB change map, populate the "all components" map, plus
	//copy the global HB change map entries for each node into the more
	//persistent one.

	// HB Started maps
	for k,v := range(cpStartMap) {
		if (v != 0) {
			allCompsMap[k] = true
		}
	}
	for k,v := range(StartMap) {
		if (v != 0) {
			cpStartMap[k] = v
			StartMap[k] = 0
			allCompsMap[k] = true
		}
	}

	// HB Restarted maps
	for k,v := range(cpRestartMap) {
		if (v != 0) {
			allCompsMap[k] = true
		}
	}
	for k,v := range(RestartMap) {
		if (v != 0) {
			cpRestartMap[k] = v
			RestartMap[k] = 0
			allCompsMap[k] = true
		}
	}

	// HB Stopped/warning maps
	for k,v := range(cpStopWarnMap) {
		if (v != 0) {
			allCompsMap[k] = true
		}
	}
	for k,v := range(StopWarnMap) {
		if (v != 0) {
			cpStopWarnMap[k] = v
			StopWarnMap[k] = 0
			allCompsMap[k] = true
		}
	}

	// HB Stopped/error maps
	for k,v := range(cpStopErrorMap) {
		if (v != 0) {
			allCompsMap[k] = true
		}
	}
	for k,v := range(StopErrorMap) {
		if (v != 0) {
			cpStopErrorMap[k] = v
			StopErrorMap[k] = 0
			allCompsMap[k] = true
		}
	}
}

// Convenience function.  For each entry in map1, clear the matching map
// entry for all other maps.  This is done after successful SM PATCH
// operations to clear the HB state maps.

func groomCompLocalMapsPOST(map1,map2,map3,map4 map[string]uint64) {
	for k,_ := range(map1) {
		map2[k] = 0
		map3[k] = 0
		map4[k] = 0
	}
}

// Convenience function.  Creates HSM BulkStateInfo data structures, one
// for each HB status change type (start/restart/stop-warn/stop-error) and
// populates default values.

func createBSI() (smjbulk_v1, smjbulk_v1, smjbulk_v1, smjbulk_v1) {
	var bsiStart,bsiRestart,bsiStopWarn,bsiStopError smjbulk_v1
	bsiStart.State = base.StateReady.String()
	bsiStart.Flag = base.FlagOK.String()
	bsiStart.ExtendedInfo.Message = "Heartbeat started"
	bsiRestart.State = base.StateReady.String()
	bsiRestart.Flag = base.FlagOK.String()
	bsiRestart.ExtendedInfo.Message = "Heartbeat restarted"
	bsiStopWarn.State = base.StateReady.String()
	bsiStopWarn.Flag = base.FlagWarning.String()
	bsiStopWarn.ExtendedInfo.Message = "Heartbeat stopped -- might be dead"
	bsiStopError.State = base.StateStandby.String()
	bsiStopError.Flag = base.FlagAlert.String()
	bsiStopError.ExtendedInfo.Message = "Heartbeat stopped -- declared dead"
	return bsiStart,bsiRestart,bsiStopWarn,bsiStopError
}

/////////////////////////////////////////////////////////////////////////////
// Thread func.  Takes the values found in the global and local/persistent 
// HB status change maps, determines which ones to place into an HSM 
// BulkStateChange data structure and then PATCH them to HSM.  If there are
// failures with the PATCH, the persistent maps will help resolve conflicts
// when more than one HB state change occurs while HSM is unavailable.
/////////////////////////////////////////////////////////////////////////////

func send_sm_req() {
	cpStartMap := make(map[string]uint64)
	cpRestartMap := make(map[string]uint64)
	cpStopWarnMap := make(map[string]uint64)
	cpStopErrorMap := make(map[string]uint64)

	for {
		//Wait for next HB scan to complete.

		if (app_params.debug_level.int_param > 1) {
			hbtdPrintf("Waiting for a Q Pop.")
		}
		qval := <-hsmUpdateQ
		if (app_params.debug_level.int_param > 1) {
			hbtdPrintf("Q Popped, val: %x.",qval)
		}
		if (qval == HSMQ_DIE) {
			hbtdPrintf("DIE message received, exiting send_sm_req().")
			break
		}
		//Copy HB state changes from the global component maps into into local 
		//maps.
		//Also build a map of all pertinent components (superset of local and
		//global maps).

		allCompsMap := make(map[string]bool)
		hbMapLock.Lock()
		groomCompLocalMapsPRE(allCompsMap,cpStartMap,cpRestartMap,cpStopWarnMap,cpStopErrorMap)
		hbMapLock.Unlock()

		//Populate local copies of the HB state maps from the global ones.
		//De-duplicate the maps.  If any component saw more than one HB
		//change, take the one with the highest sequence number.  Note that 
		//each time through the outer most for() loop (indicating a new HB 
		//scan was done) if there were any HSM send errors from the previous 
		//scan, the HB states will still be retained in local copies of the
		//maps, so we'll just add from the global state maps (not start over).
		//If the HSM data was sent OK, the local maps are cleared and new 
		//data is sent.

		bsiStart,bsiRestart,bsiStopWarn,bsiStopError := createBSI()

		for k,_ := range(allCompsMap) {
			start,_   := cpStartMap[k]
			restart,_ := cpRestartMap[k]
			swarn,_   := cpStopWarnMap[k]
			serr,_    := cpStopErrorMap[k]

			//The state change category with the highest value wins, meaning 
			//that it came in last, time-wise.

			if ((start > restart) && (start > swarn) && (start > serr)) {
				bsiStart.ComponentIDs = append(bsiStart.ComponentIDs,k)
			} else if ((restart > start) && (restart > swarn) && (restart > serr)) {
				bsiRestart.ComponentIDs = append(bsiRestart.ComponentIDs,k)
			} else if ((swarn > start) && (swarn > restart) && (swarn > serr)) {
				bsiStopWarn.ComponentIDs = append(bsiStopWarn.ComponentIDs,k)
			} else if ((serr > start) && (serr > restart) && (serr > swarn)) {
				bsiStopError.ComponentIDs = append(bsiStopError.ComponentIDs,k)
			}
		}

        //If HSM is not ready, bail until next scan.

        if !hsmReady {
			hbtdPrintf("HSM Not ready, waiting until next scan.")
			continue	//wait until next scan.
		}

		//Check each bulk operation and add a wait count, then send the SM 
		//patches, in parallel, one for each HB state change type.

		if (len(bsiStart.ComponentIDs) > 0) {
			hsmWG.Add(1)
			bsiStart.needSend = true
			go send_sm_patch(&bsiStart)
		}
		if (len(bsiRestart.ComponentIDs) > 0) {
			hsmWG.Add(1)
			bsiRestart.needSend = true
			go send_sm_patch(&bsiRestart)
		}
		if (len(bsiStopWarn.ComponentIDs) > 0) {
			hsmWG.Add(1)
			bsiStopWarn.needSend = true
			go send_sm_patch(&bsiStopWarn)
		}
		if (len(bsiStopError.ComponentIDs) > 0) {
			hsmWG.Add(1)
			bsiStopError.needSend = true
			go send_sm_patch(&bsiStopError)
		}

		if (!bsiStart.needSend && !bsiRestart.needSend &&
		    !bsiStopWarn.needSend && !bsiStopError.needSend) {
			if (app_params.debug_level.int_param > 1) {
				hbtdPrintf("Nothing to send to HSM.")
			}
			continue
		}

		//Wait until they are all complete.
		if (app_params.debug_level.int_param > 1) {
			hbtdPrintf("Waiting for HSM PATCHs to complete...")
		}
		hsmWG.Wait()
		if (app_params.debug_level.int_param > 1) {
			hbtdPrintf("PATCHs complete: %t %t %t %t",
				bsiStart.sentOK, bsiRestart.sentOK,bsiStopWarn.sentOK,
				bsiStopError.sentOK)
		}

		//See if all of the ones we sent have completed OK.  For the ones
		//that sent to SM OK, delete it's local map to start over.  Any
		//that failed, retain the local map so it can be updated on the next
		//scan.

		hbMapLock.Lock()
		if (bsiStart.needSend && bsiStart.sentOK) {
			groomCompLocalMapsPOST(cpStartMap,cpRestartMap,cpStopWarnMap,cpStopErrorMap)
			cpStartMap = make(map[string]uint64)
		}
		if (bsiRestart.needSend && bsiRestart.sentOK) {
			groomCompLocalMapsPOST(cpRestartMap,cpStartMap,cpStopWarnMap,cpStopErrorMap)
			cpRestartMap = make(map[string]uint64)
		}
		if (bsiStopWarn.needSend && bsiStopWarn.sentOK) {
			groomCompLocalMapsPOST(cpStopWarnMap,cpStartMap,cpRestartMap,cpStopErrorMap)
			cpStopWarnMap = make(map[string]uint64)
		}
		if (bsiStopError.needSend && bsiStopError.sentOK) {
			groomCompLocalMapsPOST(cpStopErrorMap,cpStartMap,cpRestartMap,cpStopWarnMap)
			cpStopErrorMap = make(map[string]uint64)
		}
		hbMapLock.Unlock()
    }
}

/////////////////////////////////////////////////////////////////////////////
// Thread function, send heartbeat status changes to the telemetry bus.
/////////////////////////////////////////////////////////////////////////////

func telemetry_handler() {
	var tmsg telemetry_json_v1

	for {
		if (app_params.use_telemetry.int_param == 0) {
			time.Sleep(5 * time.Second)
			continue
		}

		tmsg = <-telemetryQ
		tbMutex.Lock()
		if (msgbusHandle != nil) {
			jdata,err := json.Marshal(tmsg)
			if (err == nil) {
				err = msgbusHandle.MessageWrite(string(jdata))
				if (err != nil) {
					hbtdPrintln("ERROR injecting telemetry data:",err)
				}
			} else {
				hbtdPrintln("ERROR marshalling telemetry data:",err)
			}
		}
		tbMutex.Unlock()
	}
}

/////////////////////////////////////////////////////////////////////////////
// Set values in the HB status change component maps based on this heartbeat's
// information.  This is called by the heartbeat checker and when new 
// heartbeats arrive.  The maps are used when the heartbeat checker calls
// send_sm_message() to update HSM.
//
// We also will put HB change notifications into the Kafka Q.
/////////////////////////////////////////////////////////////////////////////

func hb_update_notify(hb *hbinfo, to_state int) {
	var telemsg telemetry_json_v1

	telemsg.MessageID = TELEMETRY_MESSAGE_ID
	telemsg.Id = hb.Component
	telemsg.LastHBTimeStamp = hb.Last_hb_timestamp
	hbSeq ++

	switch(to_state) {
		case HB_started:
			hbMapLock.Lock()
			StartMap[hb.Component] = hbSeq
			hbMapLock.Unlock()
			telemsg.NewState = base.StateReady.String()
			telemsg.NewFlag = base.FlagOK.String()
			telemsg.Info = "Heartbeat started."
		case HB_restarted_warn:
			hbMapLock.Lock()
			RestartMap[hb.Component] = hbSeq
			hbMapLock.Unlock()
			telemsg.NewState = base.StateReady.String()
			telemsg.NewFlag = base.FlagOK.String()
			telemsg.Info = "Heartbeat re-started."
		case HB_stopped_warn:
			hbMapLock.Lock()
			StopWarnMap[hb.Component] = hbSeq
			hbMapLock.Unlock()
			telemsg.NewState = base.StateReady.String()
			telemsg.NewFlag = base.FlagWarning.String()
			telemsg.Info = "Heartbeat stopped, node may be dead."
		case HB_stopped_error:
			hbMapLock.Lock()
			StopErrorMap[hb.Component] = hbSeq
			hbMapLock.Unlock()
			telemsg.NewState = base.StateStandby.String()
			telemsg.NewFlag = base.FlagAlert.String()
			telemsg.Info = "Heartbeat stopped, node is dead."
		default:
			hbtdPrintf("INTERNAL ERROR: UNKNOWN STATE: %d",to_state)
	}

	select {
		case telemetryQ <-telemsg:
		default:
			hbtdPrintf("INFO: Telemetry bus not accepting messages, heartbeat event not sent.")
	}
}

/////////////////////////////////////////////////////////////////////////////
// Convenience function -- re-arm the timer to call the HB checker routine.
//
// Args, return: None
/////////////////////////////////////////////////////////////////////////////

func rearm_hbcheck_timer() {
    if (app_params.check_interval.int_param > 0) {
        time.AfterFunc((time.Duration(app_params.check_interval.int_param) * time.Second),
                                     hb_checker)
    }
}

/////////////////////////////////////////////////////////////////////////////
// This function is called periodically by a timer.  It will run through the
// list of currently tracked components and send notifications of any 
// delinquencies.  Note that new HB startup notifications are sent in the
// hb_rcv() function.
//
// Args, return: None
/////////////////////////////////////////////////////////////////////////////

func hb_checker () {
    var nhb  hbinfo
    var storeit bool
    var verr error
    var now,tdiff,lhbtime int64
    var deleteKeys []string
    var updateKeys []hmetcd.Kvi_KV

    if (app_params.debug_level.int_param > 1) {
        hbtdPrintf("HB CHECKER entry.")
    }

    ncomp := 0

    // Grab the inter-process lock and get all keys/vals.  

    if (app_params.check_interval.int_param > 0) {
        if (app_params.debug_level.int_param > 1) {
            hbtdPrintf("Locking...")
        }
        tstart := time.Now()
        lckerr := kvHandle.DistTimedLock(app_params.check_interval.int_param*2)
        tfin := time.Now()
        elapsed := tfin.Sub(tstart) / time.Second
        if (int(elapsed) > (app_params.check_interval.int_param*3)) {
            hbtdPrintf("WARNING: Distributed lock acquisition attempt took %d seconds.",
            elapsed)
        }
        if (lckerr != nil) {
            //Lock is already held.  This means someone else is doing the check,
            //so we don't have to.

            if (app_params.debug_level.int_param > 1) {
                hbtdPrintf("HB checker being done elsewhere, skipping.\n")
                hbtdPrintln("  (returned:",lckerr,")")
            }
            rearm_hbcheck_timer()
            return
        }
        if (app_params.debug_level.int_param > 1) {
            hbtdPrintf("HB Checker lock acquired.")
        }
    }

    //Test code, activated by environment variable.  Causes the HB checker
    //to sleep whilst holding the lock, simulating cases where the HB checker
    //takes a long time, to verify that multi-instances will take over for 
    //each other.

    envstr := os.Getenv("HBTD_RSLEEP")
    if (envstr != "") {
        slp,_ := strconv.Atoi(envstr)
        hbtdPrintf("Sleeping for %d seconds..\n",slp)
        time.Sleep(time.Duration(slp) * time.Second)
    }

    kvlist, err := kvHandle.GetRange(HB_KEYRANGE_START,HB_KEYRANGE_END)
    if (err != nil) {
        hbtdPrintln("ERROR fetching all hbtd keys from KV store: ",err)
        kvHandle.DistUnlock() //ignore errors
        rearm_hbcheck_timer()
        return
    }

    for _,kv := range kvlist {
        //Skip special keys
        if (kv.Key == KV_PARAM_KEY) {
            continue
        }

        storeit = false
        ncomp ++

        if (app_params.debug_level.int_param > 1) {
            hbtdPrintf("Checking component: '%s'\n",kv.Key);
        }

        verr = json.Unmarshal([]byte(kv.Value),&nhb)
        if (verr != nil) {
            hbtdPrintln("ERROR unmarshalling '",kv.Value,"': ",verr)
            continue
        }

        //Get the current time.  We will get it here rather than once at the
        //beginning of this function since there can be delays in getting
        //the key/value store, and we need to be able to accurately calculate
        //the time elapsed since the HB was received.

        //TODO: maybe should compare the component with the key?  Seems like
        //that's just extra work.

        //TODO: this has 1 second resolution.  Should be enough.

        now = time.Now().Unix()

        lhbtime,_ = strconv.ParseInt(nhb.Last_hb_rcv_time,16,64)
        tdiff = now - lhbtime

        if (tdiff >= int64(app_params.errtime.int_param)) {
            if (staleKeys) {
                //This means there was a time when there was no HBTD instance
                //running.  We'll treat these the same as warnings.
                hbtdPrintf("WARNING: Heartbeat overdue %d seconds for '%s' due to HB monitoring gap; might be dead, last status: '%s'",
                    tdiff,nhb.Component,nhb.Last_hb_status)

                //Update the HB's last received time.  This will freshen the
                //stale key so if it's really still heartbeating, it will
                //succeed next time.  If the node is truly dead, it will still
                //be reported dead once the freshened time expires.
                nhb.Last_hb_rcv_time = strconv.FormatUint(uint64(time.Now().Unix()),16)

                //Send a warning to SM
                nhb.Had_warning = HB_WARN_GAP
                hb_update_notify(&nhb,HB_stopped_warn)
                storeit = true
            } else {
                hbtdPrintf("ERROR: Heartbeat overdue %d seconds for '%s' (declared dead), last status: '%s'\n",
                    tdiff,nhb.Component,nhb.Last_hb_status)

                //Send an error to SM
                hb_update_notify(&nhb,HB_stopped_error)

                //Since it's dead, take it out of the list.
                deleteKeys = append(deleteKeys,kv.Key)
                ncomp --
                continue
            }
        } else if (tdiff >= int64(app_params.warntime.int_param)) {
            if (nhb.Had_warning == HB_WARN_NONE) {
                hbtdPrintf("WARNING: Heartbeat overdue %d seconds for '%s' (might be dead), last status: '%s'\n",
                    tdiff,nhb.Component,nhb.Last_hb_status)

                //Send a warning to SM
                nhb.Had_warning = HB_WARN_NORMAL
                hb_update_notify(&nhb,HB_stopped_warn)
                storeit = true
            }
        } else {
            //HB arrived in time.  Check if there was a prior warning, and if
            //so, send a HB re-started message
            if (nhb.Had_warning == HB_WARN_NORMAL) {
                nhb.Had_warning = HB_WARN_NONE
                storeit = true
                hbtdPrintf("INFO: Heartbeat restarted for '%s'\n",nhb.Component)
                hb_update_notify(&nhb,HB_restarted_warn)
            }
        }

        if (storeit) {
            jstr,err := json.Marshal(nhb)
            if (err != nil) {
                hbtdPrintln("INTERNAL ERROR marshaling JSON for ",nhb.Component,": ",err)
            } else {
                updateKeys = append(updateKeys, hmetcd.Kvi_KV{Key: nhb.Component,
                                    Value: string(jstr),})
            }
        }
    }

    //Delete keys of dead HBs

    if (app_params.debug_level.int_param > 1) {
        hbtdPrintf("Deleting %d keys...",len(deleteKeys))
    }
    for _,dkey := range(deleteKeys) {
        verr = kvHandle.Delete(dkey)
        if (verr != nil) {
            hbtdPrintln("ERROR deleting key '",dkey,"' from KV store: ",verr)
        }
    }

    //Update keys that need updating

    if (app_params.debug_level.int_param > 1) {
        hbtdPrintf("Updating %d keys...",len(updateKeys))
    }
    for _,ukey := range(updateKeys) {
        merr := kvHandle.Store(ukey.Key,ukey.Value)
        if (merr != nil) {
            hbtdPrintf("ERROR storing key '%s': %v",ukey.Key,merr)
        }
    }

    if (app_params.debug_level.int_param > 1) {
        hbtdPrintf("Unlocking...")
    }
    if (app_params.check_interval.int_param > 0) {
        err := kvHandle.DistUnlock()
        if (err != nil) {
            hbtdPrintln("ERROR unlocking distributed lock:",err)
        }
    }

    hsmUpdateQ <-HSMQ_NEW

    if (ncomp != sg_ncomp) {
        sg_ncomp = ncomp
        hbtdPrintf("Number of components heartbeating: %d\n",ncomp)
    }

    staleKeys = false

    //Re-arm timer -- it is not periodic.  If the check interval <= 0, don't
    //re-arm (used for testing)

    rearm_hbcheck_timer()
}

// Convenience function.  Update the time stamp and associated info for this 
// component.
//
// TODO: maybe we don't mess with unmarshalling the KV HB data -- we pretty
// much just overwrite it anyway.  But, doing it this way makes it easy
// to do any data compares from the previous HB if we want to.

func updateHB(errinst, xname, timestamp, status string, w http.ResponseWriter) {
    var hbb hbinfo

    newkey := 0
    kval,kok,kerr := kvHandle.Get(xname)
    if (kerr != nil) {
        hbtdPrintf("Error reading KV key for: '%s', '%v'",xname,kerr)
    }

    if ((kok == false) || (kerr != nil)) {
        //Key does not exist.  Create it.
        newkey = 1

        hbb.Component = xname
    } else {
        //Key exists, just update the time stamp and status.

        umerr := json.Unmarshal([]byte(kval),&hbb)
        if (umerr != nil) {
            hbtdPrintln("INTERNAL ERROR unmarshalling '",kval,"': ",umerr)
            pdet := base.NewProblemDetails("about:blank",
                                           "Internal Server Error",
                                           "Error unmarshalling JSON string",
                                           errinst,http.StatusInternalServerError)
            base.SendProblemDetails(w,pdet,0)
            return
        }
    }

    hbb.Last_hb_rcv_time = strconv.FormatUint(uint64(time.Now().Unix()),16)
    hbb.Last_hb_timestamp = timestamp
    hbb.Last_hb_status = status

    //Special case: if this heartbeat record Had_warning flag shows a coverage
    //gap, set it to a normal warning so the checker handles is correctly.

    if (hbb.Had_warning == HB_WARN_GAP) {
        hbb.Had_warning = HB_WARN_NORMAL
    }

    jstr, jerr := json.Marshal(hbb)
    if (jerr != nil) {
        hbtdPrintln("INTERNAL ERROR marshaling JSON: ",jerr);
        pdet := base.NewProblemDetails("about:blank",
                                       "Internal Server Error",
                                       "Error marshalling JSON data",
                                       errinst,http.StatusInternalServerError)
        base.SendProblemDetails(w,pdet,0)
        return
    }

    merr := kvHandle.Store(xname,string(jstr))
    if (merr != nil) {
        hbtdPrintln("INTERNAL ERROR storing key ",string(jstr),": ",merr);
        pdet := base.NewProblemDetails("about:blank",
                                       "Internal Server Error",
                                       "Key/Value service store operation failed",
                                       errinst,http.StatusInternalServerError)
        base.SendProblemDetails(w,pdet,0)
        return
    }

    if (newkey != 0) {
        //Send notification of a new HB startup
        hbtdPrintf("INFO: Heartbeat started for '%s'\n",hbb.Component)
        hb_update_notify(&hbb,HB_started)
    }
}


/////////////////////////////////////////////////////////////////////////////
// Callback from the server loop when a HB request comes in.
//
/////////////////////////////////////////////////////////////////////////////

func hbRcv(w http.ResponseWriter, r *http.Request) {
    errinst := URL_HEARTBEAT

    if (r.Method != "POST") {
        hbtdPrintf("ERROR: request is not a POST.\n")
        pdet := base.NewProblemDetails("about:blank",
                                       "Invalid Request",
                                       "Only POST operations supported",
                                       errinst,http.StatusMethodNotAllowed)

        //It is required to have an "Allow:" header with this error
        w.Header().Add("Allow","POST")
        base.SendProblemDetails(w,pdet,0)
        return
    }

    var jdata hbjson_full_v1
    body,err := ioutil.ReadAll(r.Body)
    err = json.Unmarshal(body,&jdata)

    if (err != nil) {
        var v map[string]interface{}

        //The Unmarshal failed, find out which field(s) specifically failed.
        //There's no quick-n-dirty way to do this so we'll just bulldoze
        //through each field and verify it.

        errstr := "Invalid JSON data type"
        errb := json.Unmarshal(body,&v)
        if (errb != nil) {
            hbtdPrintln("Unmarshal into map[string]interface{} didn't work:",errb)
        } else {
            //Figure out what field(s) == bad and report them.  For now, they're
            //all strings.

            mtype := reflect.TypeOf(jdata)
            for i := 0; i < mtype.NumField(); i ++ {
                nm := mtype.Field(i).Name
                if (v[nm] == nil) {
                    continue
                }
                _,ok := v[nm].(string)  //for now everything is strings.
                if (!ok) {
                    errstr = fmt.Sprintf("Invalid data type in %s field",nm)
                    break
                }
            }
        }
        hbtdPrintln("Bad heartbeat JSON decode:",err);
        pdet := base.NewProblemDetails("about:blank",
                                       "Invalid Request",
                                       errstr,
                                       errinst,http.StatusBadRequest)
        base.SendProblemDetails(w,pdet,0)
        return
    }

    //Check all the fields to be sure they are valid.  TODO: we could
    //check the Component to be sure it's a valid XName, but some
    //customer might want to use their own node names and track things
    //anyway; thus, for now at least, we won't limit tracking to just
    //valid XNames.  Note that this makes it possible for typos to be
    //acceptable component names!

    ferrstr := ""
    if (jdata.Component == "") {
        ferrstr = "Missing Component field"
    } else if (jdata.Hostname == "") {
        ferrstr = "Missing Hostname field"
    } else if (jdata.NID == "") {
        ferrstr = "Missing NID field"
    } else if (jdata.Status == "") {
        ferrstr = "Missing Status field"
    } else if (jdata.Timestamp == "") {
        ferrstr = "Missing Timestamp field"
    }

    if (ferrstr != "") {
        hbtdPrintf("Incomplete heartbeat JSON: %s\n",ferrstr);
        pdet := base.NewProblemDetails("about:blank",
                                       "Invalid Request",
                                       ferrstr,
                                       errinst,http.StatusBadRequest)
        base.SendProblemDetails(w,pdet,0)
        return
    }

    //Check to be sure that certain fields' values are valid.

    if (base.GetHMSType(jdata.Component) == base.HMSTypeInvalid) {
        hbtdPrintf("Invalid XName in heartbeat JSON: %s\n",jdata.Component);
        pdet := base.NewProblemDetails("about:blank",
                                       "Invalid Request",
                                       "Invalid Component Name",
                                       errinst,http.StatusBadRequest)
        base.SendProblemDetails(w,pdet,0)
        return
    }

    _,cerr := strconv.ParseInt(jdata.NID,0,64)
    if (cerr != nil) {
        hbtdPrintf("Invalid NID in heartbeat JSON: %s\n",jdata.NID);
        pdet := base.NewProblemDetails("about:blank",
                                       "Invalid Request",
                                       "Invalid NID",
                                       errinst,http.StatusBadRequest)
        base.SendProblemDetails(w,pdet,0)
        return
    }

    if (app_params.debug_level.int_param > 0) {
        hbtdPrintf("Heartbeat: Component: %s, Host: %s, NID: %s, Status: %s, time: %s\n",
            jdata.Component,jdata.Hostname,jdata.NID,jdata.Status,
            jdata.Timestamp)
    }

    if (app_params.debug_level.int_param > 1) {
        hbtdPrintf("HB received for: '%s'",jdata.Component)
    }

    //Update the time stamp and info for this component.

    updateHB(errinst,jdata.Component,jdata.Timestamp,jdata.Status,w)
}

func hbRcvXName(w http.ResponseWriter, r *http.Request) {
    errinst := URL_HEARTBEAT
    xname := mux.Vars(r)["xname"]
    if (xname == "") {
        //Should NOT happen, but if so, grab the end of the URL
        toks := strings.Split(r.URL.Path,"/")
        xn := base.NormalizeHMSCompID(toks[len(toks)-1])
        if (xn == "") {
            //Enforce valid XName
            hbtdPrintf("ERROR: request is not a POST.\n")
            pdet := base.NewProblemDetails("about:blank",
                                       "Invalid Request",
                                       "Only POST operations supported",
                                       errinst,http.StatusMethodNotAllowed)
            base.SendProblemDetails(w,pdet,0)
            return
        }
		xname = xn
	}

    if (r.Method != "POST") {
        hbtdPrintf("ERROR: request is not a POST.\n")
        pdet := base.NewProblemDetails("about:blank",
                                       "Invalid Request",
                                       "Only POST operations supported",
                                       errinst,http.StatusMethodNotAllowed)

        //It is required to have an "Allow:" header with this error
        w.Header().Add("Allow","POST")
        base.SendProblemDetails(w,pdet,0)
        return
    }

    var jdata hbjson_v1
    body,err := ioutil.ReadAll(r.Body)
    err = json.Unmarshal(body,&jdata)

    if (err != nil) {
        var v map[string]interface{}

        //The Unmarshal failed, find out which field(s) specifically failed.
        //There's no quick-n-dirty way to do this so we'll just bulldoze
        //through each field and verify it.

        errstr := "Invalid JSON data type"
        errb := json.Unmarshal(body,&v)
        if (errb != nil) {
            hbtdPrintln("Unmarshal into map[string]interface{} didn't work:",errb)
        } else {
            //Figure out what field(s) == bad and report them.  For now, they're
            //all strings.

            mtype := reflect.TypeOf(jdata)
            for i := 0; i < mtype.NumField(); i ++ {
                nm := mtype.Field(i).Name
                if (v[nm] == nil) {
                    continue
                }
                _,ok := v[nm].(string)  //for now everything is strings.
                if (!ok) {
                    errstr = fmt.Sprintf("Invalid data type in %s field",nm)
                    break
                }
            }
        }
        hbtdPrintln("Bad heartbeat JSON decode:",err);
        pdet := base.NewProblemDetails("about:blank",
                                       "Invalid Request",
                                       errstr,
                                       errinst,http.StatusBadRequest)
        base.SendProblemDetails(w,pdet,0)
        return
    }

    //Check all the fields to be sure they are valid.  

    ferrstr := ""
    if (jdata.Status == "") {
        ferrstr = "Missing Status field"
    } else if (jdata.Timestamp == "") {
        ferrstr = "Missing Timestamp field"
    }

    if (ferrstr != "") {
        hbtdPrintf("Incomplete heartbeat JSON: %s\n",ferrstr);
        pdet := base.NewProblemDetails("about:blank",
                                       "Invalid Request",
                                       ferrstr,
                                       errinst,http.StatusBadRequest)
        base.SendProblemDetails(w,pdet,0)
        return
    }

    if (app_params.debug_level.int_param > 0) {
        hbtdPrintf("Heartbeat: Status: %s, time: %s\n",
            jdata.Status,jdata.Timestamp)
    }

    //Update the time stamp and info for this component.

    if (app_params.debug_level.int_param > 1) {
        hbtdPrintf("HB received for: '%s'",xname)
    }

    updateHB(errinst,xname,jdata.Timestamp,jdata.Status,w)
}

/////////////////////////////////////////////////////////////////////////////
// Callback from the server loop when a param GET or PATCH request comes in.
/////////////////////////////////////////////////////////////////////////////

func paramsIO(w http.ResponseWriter, r *http.Request) {
    var rparams []byte
    errinst := URL_PARAMS

    if (r.Method == "PATCH") {
        body,err := ioutil.ReadAll(r.Body)

        if (err != nil) {
            hbtdPrintln("Error on message read:",err);
            pdet := base.NewProblemDetails("about:blank",
                                           "Invalid Request",
                                           "Error reading inbound request",
                                           errinst,http.StatusBadRequest)
            base.SendProblemDetails(w,pdet,0)
            return
        }

        //OK, payload is OK.  Set the param values found in it.

        var errstrs string

        if (parse_parm_json(body,PARAM_PATCH,&errstrs) != 0) {
            hbtdPrintf("Error parsing parameter JSON: '%s'\n",errstrs)
            pdet := base.NewProblemDetails("about:blank",
                                           "Invalid Request",
                                           errstrs,
                                           errinst,http.StatusBadRequest)
            base.SendProblemDetails(w,pdet,0)
            return
        }

        //OK, if we got here, things applied correctly.  Generate a JSON
        //response with the current values of the parameters.

        if (gen_cur_param_json(&rparams) != 0) {
            pdet := base.NewProblemDetails("about:blank",
                                           "Internal Server Error",
                                           "Failed JSON marshall",
                                           errinst,http.StatusInternalServerError)
            base.SendProblemDetails(w,pdet,0)
            return
        }

        //Set this JSON blob as a key for the KV store so that all
        //instances of this service see it and use the same values of
        //parameters.

        merr := kvHandle.Store(KV_PARAM_KEY,string(rparams))
        if (merr != nil) {
            hbtdPrintln("INTERNAL ERROR storing KV params value ",
                string(rparams),": ",merr);
            pdet := base.NewProblemDetails("about:blank",
                                           "Internal Server Error",
                                           "Failed KV service STORE operation",
                                           errinst,http.StatusInternalServerError)
            base.SendProblemDetails(w,pdet,0)
            return
        }

        w.WriteHeader(http.StatusOK)
        w.Write(rparams)
    } else if (r.Method == "GET") {
        if (gen_cur_param_json(&rparams) != 0) {
            pdet := base.NewProblemDetails("about:blank",
                                           "Internal Server Error",
                                           "Failed JSON marshall",
                                           errinst,http.StatusInternalServerError)
            base.SendProblemDetails(w,pdet,0)
            return
        }
        w.Header().Set("Content-Type","application/json")
        w.WriteHeader(http.StatusOK)
        w.Write(rparams)
    } else {
        hbtdPrintf("ERROR: request is not a PATCH or a GET.\n")
        pdet := base.NewProblemDetails("about:blank",
                                       "Invalid Request",
                                       "Only PATCH and GET operations supported",
                                       errinst,http.StatusMethodNotAllowed)
        //It is required to have an "Allow:" header with this error
        w.Header().Add("Allow","GET,PATCH")
        base.SendProblemDetails(w,pdet,0)
        return
    }
}

// Convenience function, given a component name and time reference, determine
// whether that component is heartbeating.
//
// xname(in):   Name of component to check.
// now(in):     Time reference, used to calculate heartbeat state.
// errinst(in): Function name of caller (for error messaging).
// Return:      true if component is heartbeating, else false
//              Problem report on error for caller to use.

func isHeartbeating(xname string, now int64, errinst string) (bool, *base.ProblemDetails) {
	var hbb hbinfo

	kval,kok,kerr := kvHandle.Get(xname)
	if (kerr != nil) {
		pdet := base.NewProblemDetails("about:blank",
		                               "Invalid Request",
		                               fmt.Sprintf("Error retrieving key '%s'",xname),
		                               errinst,http.StatusInternalServerError)
		return false,pdet
	}
	if (kok == false) {
		return false,nil
	}

	umerr := json.Unmarshal([]byte(kval),&hbb)
	if (umerr != nil) {
		hbtdPrintln("INTERNAL ERROR unmarshalling '",kval,"': ",umerr)
		pdet := base.NewProblemDetails("about:blank",
		                               "Internal Server Error",
		                               fmt.Sprintf("Error unmarshalling JSON for key '%s'",xname),
		                               errinst,http.StatusInternalServerError)
		return false,pdet
	}

	//Get the HB record's Last_hb_rcv_time timestamp and decode it.
	//Get the current time, and compare against the error time lapse.
	//If the time lapse is >= error time, node is not heartbeating; else,
	//it is.  "Might be dead" is assumed to be still functioning.

	lhbtime,_ := strconv.ParseInt(hbb.Last_hb_rcv_time,16,64)
	tdiff := now - lhbtime
	if (tdiff >= int64(app_params.errtime.int_param)) {
		return false,nil
	}

	return true,nil
}

// Entry point for /hmi/v1/hbstates

func hbStates(w http.ResponseWriter, r *http.Request) {
	var jdata hbStatesReq
	var rspData hbStatesRsp
	var rspSingle hbSingleStateRsp

	errinst := URL_HB_STATES
	body,err := ioutil.ReadAll(r.Body)

	if (err != nil) {
		hbtdPrintln("Error on message read:",err);
		pdet := base.NewProblemDetails("about:blank",
		                               "Invalid Request",
		                               "Error reading inbound request",
		                               errinst,http.StatusBadRequest)
		                               base.SendProblemDetails(w,pdet,0)
		return
	}

	err = json.Unmarshal(body,&jdata)
	if (err != nil) {
		hbtdPrintf("Error unmarshalling HB state req data: %v",err)
		pdet := base.NewProblemDetails("about:blank",
		                               "Invalid Request",
		                               "Error unmarshalling inbound request",
		                               errinst,http.StatusBadRequest)
		                               base.SendProblemDetails(w,pdet,0)
		return
	}

	now := time.Now().Unix()

	for _,comp := range(jdata.XNames) {
		isHB,pdet := isHeartbeating(comp,now,errinst)

		if (pdet != nil) {
			base.SendProblemDetails(w,pdet,0)
			return
		}

		rspSingle.XName = comp
		rspSingle.Heartbeating = isHB
		rspData.HBStates = append(rspData.HBStates,rspSingle)
	}

	ba,baerr := json.Marshal(&rspData)
	if (baerr != nil) {
		hbtdPrintf("INTERNAL ERROR marshalling rsp data: %v",baerr)
		pdet := base.NewProblemDetails("about:blank",
		                               "Internal Server Error",
		                               "Error marshalling JSON return data",
		                               errinst,http.StatusInternalServerError)
		base.SendProblemDetails(w,pdet,0)
		return
	}
	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(ba)
}

// Entry point for /hmi/v1/hbstate/{xname}

func hbStateSingle(w http.ResponseWriter, r *http.Request) {
	var rspSingle hbSingleStateRsp

	vars := mux.Vars(r)
	targ := base.NormalizeHMSCompID(vars["xname"])
	errinst := URL_HB_STATE+"/"+targ
	now := time.Now().Unix()

	isHB,pdet := isHeartbeating(targ,now,errinst)

	if (pdet != nil) {
		base.SendProblemDetails(w,pdet,0)
		return
	}

	rspSingle.XName = targ
	rspSingle.Heartbeating = isHB

	ba,baerr := json.Marshal(&rspSingle)
	if (baerr != nil) {
		hbtdPrintf("INTERNAL ERROR marshalling rsp data: %v",baerr)
		pdet := base.NewProblemDetails("about:blank",
		                               "Internal Server Error",
		                               "Error marshalling JSON return data",
		                               errinst,http.StatusInternalServerError)
		base.SendProblemDetails(w,pdet,0)
		return
	}
	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(ba)
}

