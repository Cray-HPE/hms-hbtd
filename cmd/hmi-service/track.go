// Copyright 2018-2020 Hewlett Packard Enterprise Development LP


package main

import (
    "net/http"
    "encoding/json"
    "time"
    "io/ioutil"
    "bytes"
    "strconv"
    "stash.us.cray.com/HMS/hms-base"
    "reflect"
    "fmt"
    "os"

    "github.com/gorilla/mux"
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

type hbjson_v1 struct {
    Component string `json:"Component"`
    Hostname string  `json:"Hostname"`
    NID string       `json:"NID"`
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

type smjcomp_v1 struct {
    State string `json:"State"`
    Flag string  `json:"Flag"`
    ExtendedInfo smjson_einfo
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

const UTEST_URL = "__UNIT_TEST_URL__"


/////////////////////////////////////////////////////////////////////////////
// Global variables
/////////////////////////////////////////////////////////////////////////////

// Chan/async Qs

var sm_asyncQ = make(chan sminfo, 50000)
var hsmQ = make(chan smjcomp_v1, 50000)
var telemetryQ = make(chan telemetry_json_v1, 50000)

// Used to track the number of components currently tracked

var sg_ncomp = 0


/////////////////////////////////////////////////////////////////////////////
// Send a patch to the State Mgr to set heartbeat info.
// TODO: do we want a persistent Request and/or Client and just re-use it?
//
// data(in): Byte array containing marshal'd JSON with the SM PATCH data.
// Return:   None
/////////////////////////////////////////////////////////////////////////////

func send_sm_patch(smjinfo smjcomp_v1) int {
    barr,err := json.Marshal(smjinfo)
    if (err != nil) {
        hbtdPrintln("INTERNAL ERROR marshalling SM info:",err)
        return -1
    }

    url := app_params.statemgr_url.string_param

    if (smjinfo.ExtendedInfo.Message != UTEST_URL) {
        url = url + "/" + SM_URL_MID + "/" +
           smjinfo.ExtendedInfo.Id + "/" + SM_URL_SUFFIX
	}

    if (app_params.debug_level.int_param > 1) {
        hbtdPrintf("Sending PATCH to State Mgr URL: '%s', Data: '%s'",
            url,string(barr))
    }

    //Don't actually send anything to the SM if we're in "--nosm" mode.

    if (app_params.nosm.int_param != 0) {
        return 0
    }

    // Make PATCH requests this way since http.Client has no Patch() method.

    req,_ := http.NewRequest("PATCH", url, bytes.NewBuffer(barr))
    req.Header.Set("Content-Type","application/json")

    rsp,err := htrans.client.Do(req)

    if (err != nil) {
        hbtdPrintln("ERROR sending PATCH to SM:",err)
        return -1
    } else {
        defer rsp.Body.Close()
        if ((rsp.StatusCode == http.StatusOK) ||
            (rsp.StatusCode == http.StatusNoContent) ||
            (rsp.StatusCode == http.StatusAccepted)) {

            if (app_params.debug_level.int_param > 0) {
                hbtdPrintln("SUCCESS sending PATCH to SM, response:",rsp)
            }
        } else if (rsp.StatusCode == http.StatusNotFound) {
            return http.StatusNotFound
        } else {
            hbtdPrintln("ERROR response from State Manager:",rsp.Status,"Error code:",rsp.StatusCode)
            return -1
        }
    }

    return 0
}

/////////////////////////////////////////////////////////////////////////////
// Send a PATCH request to the State Manager.   Perform retries as needed.
//
// data(in):  PATCH data in JSON format to send to State Mgr.
// Return:    0 on  success, -1 on error (retries exhausted).
/////////////////////////////////////////////////////////////////////////////

func send_sm_req() {
    var rval int
	var smjinfo smjcomp_v1

	for {
		smjinfo = <- hsmQ

        //This is for test purposes only.

        if (smjinfo.Flag == "DIE") {
            hbtdPrintf("Aborting send SM req loop.\n")
            return
        }

        //If HSM is not ready, wait until it's ready.  Can't process anything in
        //that case.  If it never goes ready, we're stuck.  HBTD is no good
        //without the HSM.

        for !hsmReady {
            time.Sleep(5 * time.Second)
        }

        //Send PATCH.  If it returns an error and if the error is 404, retry a
        //few times (corner/gap case).  If it's any other error, HSM is on the
        //fritz, retry forever there too.

        maxTry := app_params.statemgr_retries.int_param

        for {
            rval = send_sm_patch(smjinfo)
            if (rval == 0) {
                break
            } else if (rval == http.StatusNotFound) {
                //Component not found.  Retry a few times, then give up.
                if (maxTry == 0) {
                    hbtdPrintf("Component not found in HSM (404): %s, giving up.",
                        smjinfo.ExtendedInfo.Id)
                    break
                }
                maxTry --
           }

            //Else, HSM is out to lunch.  Retry until it works
            time.Sleep(1 * time.Second)
        }
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
// This is a thread which will process the sending of messages to the State
// Manager.  We don't want to block on this, ever, so we will use a buffered
// channel to send requests from the main thread, and process them in this
// thread.
//
// This function will perform a PATCH call into the State Mgr to set a warning
// or error flag for the component in question.  Along with this will be
// info about the heartbeat (status).
//
// Note that we use the SM's generic state change URL and send an "array"
// (we only send one) of components to it.  We could specifically target aa
// URL for the given component, but that requires URL fiddling.  This way
// works for any number of components.  For now we only send one, but could
// "bunch them up" at a later time if we need to.
//
// TODO: is it better to have a go routine per SM update request, or a single
// go routine to handle all of them?  I think the latter, otherwise SM could
// get pummelled by lots of simultaneous requests.  But, that shouldn't happen
// very often.  Which is better?
//
// Args, return: None
/////////////////////////////////////////////////////////////////////////////

func send_sm_message() {
    var smstuff sminfo
    var smjinfo smjcomp_v1
    var telemsg telemetry_json_v1
    var hbMsg string

    telemsg.MessageID = TELEMETRY_MESSAGE_ID

    for {
        smstuff = <-sm_asyncQ

        //This is for test purposes only.

        if (smstuff.to_state == HB_quit) {
            hbtdPrintf("Aborting SM loop.\n")
            return
        }

        hbMsg = fmt.Sprintf("Sending a notification to SM: '%s' --",
                                        smstuff.component)
        smjinfo.ExtendedInfo.Id = smstuff.component
        smjinfo.State = base.StateUnknown.String()
        smjinfo.Flag = base.FlagUnknown.String()
        smjinfo.ExtendedInfo.Flag = base.FlagUnknown.String()
        telemsg.Id = smstuff.component
        telemsg.LastHBTimeStamp = smstuff.last_hb_timestamp

        switch (smstuff.to_state) {
            case HB_started:
                hbMsg += fmt.Sprintf("STARTED.");
                smjinfo.Flag = base.FlagOK.String()
                smjinfo.State = base.StateReady.String()
                smjinfo.ExtendedInfo.Flag = base.FlagOK.String()
                smjinfo.ExtendedInfo.Message = "Heartbeat started."
                telemsg.NewState = smjinfo.State
                telemsg.NewFlag = smjinfo.Flag
                telemsg.Info = smjinfo.ExtendedInfo.Message
                break
            case HB_restarted_warn:
                hbMsg += fmt.Sprintf("RESTARTED: WARNING.");
                smjinfo.Flag = base.FlagOK.String()
                smjinfo.State = base.StateReady.String()
                smjinfo.ExtendedInfo.Flag = base.FlagOK.String()
                smjinfo.ExtendedInfo.Message = "Heartbeat re-started."
                break
            case HB_stopped_warn:
                hbMsg += fmt.Sprintf("STOPPED: WARNING.");
                smjinfo.Flag = base.FlagWarning.String()
                smjinfo.State = base.StateReady.String()
                smjinfo.ExtendedInfo.Flag = base.FlagWarning.String()
                smjinfo.ExtendedInfo.Message = "Heartbeat stopped, node may be dead."
                break
            case HB_stopped_error:
                hbMsg += fmt.Sprintf("STOPPED: ERROR.");
                smjinfo.Flag = base.FlagAlert.String()
                smjinfo.State = base.StateStandby.String() //TODO: is this correct?
                smjinfo.ExtendedInfo.Flag = base.FlagAlert.String()
                smjinfo.ExtendedInfo.Message = "Heartbeat stopped, node is dead."
                break
            default:
                hbMsg += fmt.Sprintf("INTERNAL ERROR: UNKNOWN STATE: %d",smstuff.to_state)
                hbtdPrintf(hbMsg)
                continue    //skip this one
        }
        hbtdPrintf("%s",hbMsg);

        //Send PATCH to HSM

        select {
            case hsmQ <- smjinfo:
            default:
                hbtdPrintf("WARNING: HSM Operation Queue is full! Can't send state update for %s",
                    smstuff.component)
        }

        //Send to telemetry bus if telemetry is turned on

        telemsg.NewState = smjinfo.State
        telemsg.NewFlag = smjinfo.Flag
        telemsg.Info = smjinfo.ExtendedInfo.Message
        select {
            case telemetryQ <- telemsg:
            default:
                hbtdPrintf("INFO: Telemetry bus not accepting messages, heartbeat event not sent.")
        }
    }
}

/////////////////////////////////////////////////////////////////////////////
// Send a message to the state manager notification worker thread to send
// a message.  This function should never block!
//
// NOTE: this is the ONLY function that can put messages into the Q/chan!!.
//
// Args, return: None
/////////////////////////////////////////////////////////////////////////////

func state_mgr_notify(hb *hbinfo, to_state int) {
    smdata := sminfo{hb.Component,hb.Last_hb_status,to_state,hb.Last_hb_timestamp}
    select {
        case sm_asyncQ <- smdata:
        default:
            hbtdPrintf("WARNING: HSM Update Queue is full! Can't send state update for %s",
                hb.Component)
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

    ncomp := 0

    // Grab the inter-process lock and get all keys/vals.  

    if (app_params.check_interval.int_param > 0) {
        lckerr := kvHandle.DistTimedLock(app_params.check_interval.int_param*2)
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
                state_mgr_notify(&nhb,HB_stopped_warn)
                storeit = true
            } else {
                hbtdPrintf("ERROR: Heartbeat overdue %d seconds for '%s' (declared dead), last status: '%s'\n",
                    tdiff,nhb.Component,nhb.Last_hb_status)

                //Send an error to SM
                state_mgr_notify(&nhb,HB_stopped_error)

                //Since it's dead, take it out of the list.
                verr = kvHandle.Delete(kv.Key)
                if (verr != nil) {
                    hbtdPrintln("ERROR deleting key '",kv.Key,"' from KV store: ",verr)
                }
                ncomp --
                continue
            }
        } else if (tdiff >= int64(app_params.warntime.int_param)) {
            if (nhb.Had_warning == HB_WARN_NONE) {
                hbtdPrintf("WARNING: Heartbeat overdue %d seconds for '%s' (might be dead), last status: '%s'\n",
                    tdiff,nhb.Component,nhb.Last_hb_status)

                //Send a warning to SM
                nhb.Had_warning = HB_WARN_NORMAL
                state_mgr_notify(&nhb,HB_stopped_warn)
                storeit = true
            }
        } else {
            //HB arrived in time.  Check if there was a prior warning, and if
            //so, send a HB re-started message
            if (nhb.Had_warning == HB_WARN_NORMAL) {
                nhb.Had_warning = HB_WARN_NONE
                storeit = true
                state_mgr_notify(&nhb,HB_restarted_warn)
            }
        }

        if (storeit) {
            jstr,err := json.Marshal(nhb)
            if (err != nil) {
                hbtdPrintln("INTERNAL ERROR marshaling JSON for ",nhb.Component,": ",err)
            } else {
                merr := kvHandle.Store(nhb.Component,string(jstr))
                if (merr != nil) {
                    hbtdPrintln("INTERNAL ERROR storing key ",string(jstr),": ",merr);
                }
            }
        }
    }

    if (app_params.check_interval.int_param > 0) {
        err := kvHandle.DistUnlock()
        if (err != nil) {
            hbtdPrintln("ERROR unlocking distributed lock:",err)
        }
    }

    if (ncomp != sg_ncomp) {
        sg_ncomp = ncomp
        hbtdPrintf("Number of components heartbeating: %d\n",ncomp)
    }

    staleKeys = false

    //Re-arm timer -- it is not periodic.  If the check interval <= 0, don't
    //re-arm (used for testing)

    rearm_hbcheck_timer()
}

/////////////////////////////////////////////////////////////////////////////
// Callback from the server loop when a HB request comes in.
//
/////////////////////////////////////////////////////////////////////////////

func hbRcv(w http.ResponseWriter, r *http.Request) {
    var hbb hbinfo

    errinst := "/"+URL_HEARTBEAT

    //TODO: this logic will change if we need to support GET operations.

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

    //We have everything we need.  Update the time stamp for this component
    //and also update the information sent by the component.  If the KV store
    //service is slow or stalls, it won't interfere with other HB arrivals since
    //this is a parallel GO routine.
    //
    //TODO: maybe we don't mess with unmarshalling the KV HB data -- we pretty
    //much just overwrite it anyway.  But, doing it this way makes it easy
    //to do any data compares from the previous HB if we want to.

    newkey := 0
    kval,kok,kerr := kvHandle.Get(jdata.Component)

    if ((kok == false) || (kerr != nil)) {
        //Key does not exist.  Create it.
        newkey = 1

        hbb.Component = jdata.Component
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
    hbb.Last_hb_timestamp = jdata.Timestamp
    if (jdata.Status != "") {
        hbb.Last_hb_status = jdata.Status
    } else {
        hbb.Last_hb_status = ""
    }

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

    merr := kvHandle.Store(jdata.Component,string(jstr))
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
        state_mgr_notify(&hbb,HB_started)
    }
}

/////////////////////////////////////////////////////////////////////////////
// Callback from the server loop when a param GET or PATCH request comes in.
/////////////////////////////////////////////////////////////////////////////

func paramsIO(w http.ResponseWriter, r *http.Request) {
    var rparams []byte
    errinst := "/"+URL_PARAMS

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

	errinst := "/"+URL_HB_STATES
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
	errinst := "/"+URL_HB_STATE+"/"+targ
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

