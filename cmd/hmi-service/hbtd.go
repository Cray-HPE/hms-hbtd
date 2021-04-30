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



// This is the main function and some support functions for the
// Shasta node heartbeat tracker.
//
// The heartbeat tracker will listen for heartbeat messages from
// nodes.  When it receives one with no prior heartbeat history,
// a "heartbeat started" notification will be sent to State Manager.
// After that, when the heartbeat stops, a "heartbeat stopped" message
// will be sent, if it was not expected.  Nodes will send an "I'm going
// away" heartbeat message when they are being taken down.
//

package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"path"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"stash.us.cray.com/HMS/hms-base"
	"stash.us.cray.com/HMS/hms-hmetcd"
	"stash.us.cray.com/HMS/hms-msgbus"
)

/////////////////////////////////////////////////////////////////////////////
// Data structures
/////////////////////////////////////////////////////////////////////////////

// Application parameters.

type app_param struct {
	name         string
	int_param    int
	string_param string
}

type op_params struct {
	debug_level      app_param
	nosm             app_param
	use_telemetry    app_param
	telemetry_host   app_param
	warntime         app_param
	errtime          app_param
	port             app_param //set at startup, not runtime changeable
	kv_url           app_param
	check_interval   app_param
	statemgr_url     app_param
	statemgr_timeout app_param
	statemgr_retries app_param
	clear_on_gap     app_param
}

// For parsing/unmarshalling a JSON parameter file.  Can't combine with
// op_params since unmarshalling the .ini file would overwrite the cmdline
// args.

type inidata struct {
	Debug          string `json:"Debug"`
	Nosm           string `json:"Nosm"`
	Use_telemetry  string `json:"Use_telemetry"`
	Telemetry_host string `json:"Telemetry_host"`
	Warntime       string `json:"Warntime"`
	Errtime        string `json:"Errtime"`
	Port           string `json:"Port"`
	Kv_url         string `json:"Kv_url"`
	Interval       string `json:"Interval"`
	Sm_url         string `json:"Sm_url"`
	Sm_timeout     string `json:"Sm_timeout"`
	Sm_retries     string `json:"Sm_retries"`
}

// HB server URL segment description.

type url_desc struct {
	url_prefix  string
	url_root    string
	url_version string
	url_port    string
	hostname    string
	fdqn        string
	full_url    string
}


type httpTrans struct {
	transport *http.Transport
	client    *http.Client
}

/////////////////////////////////////////////////////////////////////////////
// Constants and enums
/////////////////////////////////////////////////////////////////////////////

// State Manager component state PATCH operation URL.  Note that this is
// configurable via cmdline/env vars, but the default one should probably
// reflect the real world default.

const (
	SM_URL_BASE   = "http://localhost:27779/hsm/v1"
	SM_URL_MID    = "State/Components"
	SM_URL_SUFFIX = "BulkStateData"
	SM_URL_READY  = "service/ready"
	SM_RETRIES    = 3
	SM_TIMEOUT    = 10

	HBTD_NAME         = "hbtd"
	KV_URL_BASE       = "https://localhost:2379"
	KV_PARAM_KEY      = "params"
	HBTD_HEALTH_KEY   = "HBTD_HEALTH_KEY"
	HBTD_HEALTH_OK    = "HBTD_OK"
	HB_KEYRANGE_START = "x0"
	HB_KEYRANGE_END   = "xz"
	PARAM_START       = 1
	PARAM_PATCH       = 2
	PARAM_SYNC        = 3

	UNSTR = "xxx"
	UNINT = -1

	HBTD_LIFE_KEY_PRE   = "hbtd_lifekey-"
	HBTD_LIFE_KEY_START = 0
	HBTD_LIFE_KEY_END   = (^uint32(0) >> 1)

	URL_PORT      = "28500"
)

const HB_MSGBUS_TOPIC = "CrayHMSHeartbeatNotifications"

/////////////////////////////////////////////////////////////////////////////
// Global variables
/////////////////////////////////////////////////////////////////////////////

// Application parameters, settable via cmdline or env vars

var app_params op_params

// Message bus connection

var msgbusConfig = msgbus.MsgBusConfig{BusTech: msgbus.BusTechKafka,
	Blocking:       msgbus.NonBlocking,
	Direction:      msgbus.BusWriter,
	ConnectRetries: 10,
	Topic:          HB_MSGBUS_TOPIC,
}

var serviceName string
var msgbusHandle msgbus.MsgBusIO = nil
var kvHandle hmetcd.Kvi
var tbMutex *sync.Mutex = &sync.Mutex{}
var htrans httpTrans
var server_url_port = URL_PORT
var staleKeys = false
var Running = true

// This will be used for output.  We will normally use the built-in
// functions, but we want to be able to override them for test purposes.

var hbtdPrintf = log.Printf
var hbtdPrintln = log.Println

/////////////////////////////////////////////////////////////////////////////
// Initialize default values to the application parameters.
/////////////////////////////////////////////////////////////////////////////

func initAppParams() {
	app_params = op_params{
		debug_level:      app_param{name: "debug", int_param: 0},
		nosm:             app_param{name: "nosm", int_param: 0},
		use_telemetry:    app_param{name: "use_telemetry", int_param: 1},
		telemetry_host:   app_param{name: "telemetry_host", string_param: ""},
		warntime:         app_param{name: "warntime", int_param: 10},
		errtime:          app_param{name: "errtime", int_param: 30},
		check_interval:   app_param{name: "interval", int_param: 5},
		port:             app_param{name: "port", string_param: URL_PORT},
		kv_url:           app_param{name: "kv_url", string_param: KV_URL_BASE},
		statemgr_url:     app_param{name: "sm_url", string_param: SM_URL_BASE},
		statemgr_retries: app_param{name: "sm_retries", int_param: SM_RETRIES},
		statemgr_timeout: app_param{name: "sm_timeout", int_param: SM_TIMEOUT},
		clear_on_gap:     app_param{name: "clear_on_gap", int_param: 0},
	}
}


/////////////////////////////////////////////////////////////////////////////
// Print function to be used when the time stamp stuff is not needed.
/////////////////////////////////////////////////////////////////////////////

func plainPrint(format string, a ...interface{}) {
	fmt.Printf(format,a...)
}

/////////////////////////////////////////////////////////////////////////////
// Print help text
//
// Args, return: none.
/////////////////////////////////////////////////////////////////////////////

func printHelp() {
	hbtdPrintf("Usage: %s [options]\n\n", path.Base(os.Args[0]))
	hbtdPrintf("  --help                      Help text.\n")
	hbtdPrintf("  --debug=num                 Debug level.  (Default: 0)\n")
	hbtdPrintf("  --use_telemetry=yes|no      Inject notifications into message.\n")
	hbtdPrintf("                              bus. (Default: yes)\n")
	hbtdPrintf("  --telemetry_host=h:p:t      Hostname:port:topic of telemetry service\n")
	hbtdPrintf("  --warntime=secs             Seconds before sending a warning of\n")
	hbtdPrintf("                              node heartbeat failure.  \n")
	hbtdPrintf("                              (Default: 10 seconds)\n")
	hbtdPrintf("  --errtime=secs              Seconds before sending an error of\n")
	hbtdPrintf("                              node heartbeat failure.  \n")
	hbtdPrintf("                              (Default: 30 seconds)\n")
	hbtdPrintf("  --interval=secs             Heartbeat check interval.\n")
	hbtdPrintf("                              (Default: 5 seconds)\n")
	hbtdPrintf("  --port=num                  HTTPS port to listen on.  (Default: %s)\n",
		URL_PORT)
	hbtdPrintf("  --kv_url=url                Key-Value service 'base' URL..  (Default: %s)\n",
		KV_URL_BASE)
	hbtdPrintf("  --sm_url=url                State Manager 'base' URL.  (Default: %s)\n",
		SM_URL_BASE)
	hbtdPrintf("  --sm_retries=num            Number of State Manager access retries. (Default: %d)\n",
		SM_RETRIES)
	hbtdPrintf("  --sm_timeout=secs           State Manager access timeout. (Default: %d)\n",
		SM_TIMEOUT)
	hbtdPrintf("  --nosm                      Don't contact State Manager (for testing).\n")
	hbtdPrintf("\n")
}

/////////////////////////////////////////////////////////////////////////////
// Convenience function to parse a host:port specification.
//
// hspec(in): Host:port specification.
// Return:    Hostname; Port number; Topic; Error code on failure, or nil.
/////////////////////////////////////////////////////////////////////////////

func get_telemetry_host(hspec string) (string, int, string, error) {
	var err error

	toks := strings.Split(hspec, ":")
	if len(toks) != 3 {
		err = fmt.Errorf("Invalid telemetry host specification '%s', should be host:port:topic format.",
			hspec)
		return "", 0, "", err
	}
	port, perr := strconv.Atoi(toks[1])
	if perr != nil {
		err = fmt.Errorf("Invalid port specification '%s', must be numeric.", toks[1])
		return "", 0, "", err
	}

	return toks[0], port, toks[2], nil
}

/////////////////////////////////////////////////////////////////////////////
// Generate a byte array containing JSON representing the current configurable
// parameters values.
//
// paramstr(out): Byte array containing current params JSON data.
// Return:        0 on success, -1 on error.
/////////////////////////////////////////////////////////////////////////////

func gen_cur_param_json(paramstr *[]byte) int {
	var pj inidata

	pj.Debug = strconv.Itoa(app_params.debug_level.int_param)
	pj.Nosm = strconv.Itoa(app_params.nosm.int_param)
	pj.Use_telemetry = strconv.Itoa(app_params.use_telemetry.int_param)
	pj.Telemetry_host = app_params.telemetry_host.string_param
	pj.Warntime = strconv.Itoa(app_params.warntime.int_param)
	pj.Errtime = strconv.Itoa(app_params.errtime.int_param)
	pj.Port = app_params.port.string_param
	pj.Kv_url = app_params.kv_url.string_param
	pj.Interval = strconv.Itoa(app_params.check_interval.int_param)
	pj.Sm_url = app_params.statemgr_url.string_param
	pj.Sm_timeout = strconv.Itoa(app_params.statemgr_timeout.int_param)
	pj.Sm_retries = strconv.Itoa(app_params.statemgr_retries.int_param)

	ba, err := json.Marshal(pj)
	if err != nil {
		hbtdPrintln("INTERNAL ERROR marshalling json:", err)
		return -1
	}
	*paramstr = ba
	return 0
}

/////////////////////////////////////////////////////////////////////////////
// Parse the command line arguments.  Note that command line args always "win"
// over env vars.
//
// Args, return: none.
/////////////////////////////////////////////////////////////////////////////

func parse_cmd_line() {
	helpP := flag.Bool("help", false, "Help text")
	dlevP := flag.Int(app_params.debug_level.name, UNINT, "Debug level")
	teleP := flag.String(app_params.use_telemetry.name, UNSTR, "Inject notifications into telemetry bus")
	thostP := flag.String(app_params.telemetry_host.name, UNSTR, "Telemetry service host:port")
	warnP := flag.Int(app_params.warntime.name, UNINT, "Seconds before sending a warning")
	errP := flag.Int(app_params.errtime.name, UNINT, "Seconds before sending an error.")
	checkP := flag.Int(app_params.check_interval.name, UNINT, "Seconds between heartbeat checks.")
	portP := flag.String(app_params.port.name, URL_PORT, "URL port to listen on.")
	kvurlP := flag.String(app_params.kv_url.name, KV_URL_BASE, "Key/Value service URL.")
	smurlP := flag.String(app_params.statemgr_url.name, UNSTR, "State Mgr URL to send to.")
	smtryP := flag.Int(app_params.statemgr_retries.name, UNINT, "State Mgr retry max count.")
	smtoP := flag.Int(app_params.statemgr_timeout.name, UNINT, "State Mgr timeout duration.")
	nosmP := flag.Bool(app_params.nosm.name, false, "Don't contact State Manager")

	flag.Parse()

	if *helpP != false {
		hbtdPrintf = plainPrint
		printHelp()
		os.Exit(0)
	}

	nosmi := 0
	if *nosmP == true {
		nosmi = 1
	}
	tvars := op_params{debug_level: app_param{name: "", int_param: *dlevP, string_param: ""},
		nosm:             app_param{name: "", int_param: nosmi, string_param: ""},
		use_telemetry:    app_param{name: "", int_param: 0, string_param: *teleP},
		telemetry_host:   app_param{name: "", int_param: 0, string_param: *thostP},
		warntime:         app_param{name: "", int_param: *warnP, string_param: ""},
		errtime:          app_param{name: "", int_param: *errP, string_param: ""},
		check_interval:   app_param{name: "", int_param: *checkP, string_param: ""},
		port:             app_param{name: "", int_param: 0, string_param: *portP},
		kv_url:           app_param{name: "", int_param: 0, string_param: *kvurlP},
		statemgr_url:     app_param{name: "", int_param: 0, string_param: *smurlP},
		statemgr_retries: app_param{name: "", int_param: *smtryP, string_param: ""},
		statemgr_timeout: app_param{name: "", int_param: *smtoP, string_param: ""},
	}

	parse_cmdline_params(tvars)
}

func parse_cmdline_params(tvars op_params) {
	if tvars.nosm.int_param != 0 {
		app_params.nosm.int_param = 1
	}

	if tvars.debug_level.int_param != UNINT {
		if tvars.debug_level.int_param <= 0 {
			app_params.debug_level.int_param = 0
		} else {
			app_params.debug_level.int_param = tvars.debug_level.int_param
		}
	}

	if tvars.use_telemetry.string_param != UNSTR {
		lcut := strings.ToLower(tvars.use_telemetry.string_param)
		if (lcut == "0") || (lcut == "no") || (lcut == "off") || (lcut == "false") {
			app_params.use_telemetry.int_param = 0
		} else if (lcut == "1") || (lcut == "yes") || (lcut == "on") || (lcut == "true") {
			app_params.use_telemetry.int_param = 1
		} else {
			hbtdPrintf("ERROR, parameter '%s' with unknown value '%s', setting to 0.\n",
				app_params.use_telemetry.name, tvars.use_telemetry.string_param)
			app_params.use_telemetry.int_param = 0
		}
	}

	if tvars.telemetry_host.string_param != UNSTR {
		_, _, _, hperr := get_telemetry_host(tvars.telemetry_host.string_param)
		if hperr != nil {
			hbtdPrintln(hperr)
		} else {
			app_params.telemetry_host.string_param = tvars.telemetry_host.string_param
		}
	}

	if tvars.warntime.int_param != UNINT {
		if tvars.warntime.int_param <= 0 {
			app_params.warntime.int_param = 0
		} else {
			app_params.warntime.int_param = tvars.warntime.int_param
		}
	}

	if tvars.errtime.int_param != UNINT {
		if tvars.errtime.int_param <= 0 {
			app_params.errtime.int_param = 0
		} else {
			app_params.errtime.int_param = tvars.errtime.int_param
		}
	}

	if tvars.check_interval.int_param != UNINT {
		if tvars.check_interval.int_param <= 0 {
			app_params.check_interval.int_param = 0
		} else {
			app_params.check_interval.int_param = tvars.check_interval.int_param
		}
	}

	if tvars.port.string_param != URL_PORT {
		_, err := strconv.ParseUint(tvars.port.string_param, 0, 32)
		if err != nil {
			hbtdPrintf("ERROR: invalid port number '%s'.\n",
				tvars.port.string_param)
		} else {
			app_params.port.string_param = tvars.port.string_param
			server_url_port = app_params.port.string_param
		}
	}

	if tvars.kv_url.string_param != UNSTR {
		app_params.kv_url.string_param = tvars.kv_url.string_param
	}

	if tvars.statemgr_url.string_param != UNSTR {
		app_params.statemgr_url.string_param = tvars.statemgr_url.string_param
	}

	if tvars.statemgr_retries.int_param != UNINT {
		if tvars.statemgr_retries.int_param <= 0 {
			app_params.statemgr_retries.int_param = 1
		} else {
			app_params.statemgr_retries.int_param = tvars.statemgr_retries.int_param
		}
	}

	if tvars.statemgr_timeout.int_param != UNINT {
		if tvars.statemgr_timeout.int_param <= 0 {
			app_params.statemgr_timeout.int_param = 1
		} else {
			app_params.statemgr_timeout.int_param = tvars.statemgr_timeout.int_param
		}
	}
}

/////////////////////////////////////////////////////////////////////////////
// Convenience function to parse an integer-based environment variable.
//
// envvar(in): Env variable string
// pval(out):  Ptr to an integer to hold the result.
// Return:     None.
/////////////////////////////////////////////////////////////////////////////

func __env_parse_int(envvar string, pval *int) {
	var val string
	if val = os.Getenv(envvar); val != "" {
		ival, err := strconv.ParseUint(val, 0, 64)
		if err != nil {
			hbtdPrintf("ERROR: invalid %s value '%s'.\n", envvar, val)
		} else {
			*pval = int(ival)
		}
	}
}

/////////////////////////////////////////////////////////////////////////////
// Convenience function to parse a boolean-based environment variable.
//
// envvar(in): Env variable string
// pval(out):  Ptr to an integer to hold the result.
// Return:     None.
/////////////////////////////////////////////////////////////////////////////

func __env_parse_bool(envvar string, pval *int) {
	var val string
	if val = os.Getenv(envvar); val != "" {
		lcut := strings.ToLower(val)
		if (lcut == "0") || (lcut == "no") || (lcut == "off") || (lcut == "false") {
			*pval = 0
		} else if (lcut == "1") || (lcut == "yes") || (lcut == "on") || (lcut == "true") {
			*pval = 1
		} else {
			hbtdPrintf("ERROR: invalid %s value '%s'.\n", envvar, val)
		}
	}
}

/////////////////////////////////////////////////////////////////////////////
// Convenience function to parse a string-based environment variable.
//
// envvar(in): Env variable string
// pval(out):  Ptr to an integer to hold the result.
// Return:     None.
/////////////////////////////////////////////////////////////////////////////

func __env_parse_string(envvar string, pval *string) {
	var val string
	if val = os.Getenv(envvar); val != "" {
		*pval = val
	}
}

/////////////////////////////////////////////////////////////////////////////
// Fetch env vars.  This is done first.
//
// Args, return: None.
/////////////////////////////////////////////////////////////////////////////

func parse_env_vars() {
	__env_parse_int("HBTD_DEBUG", &app_params.debug_level.int_param)
	__env_parse_bool("HBTD_NOSM", &app_params.nosm.int_param)
	__env_parse_bool("HBTD_USE_TELEMETRY", &app_params.use_telemetry.int_param)
	__env_parse_string("HBTD_TELEMETRY_HOST", &app_params.telemetry_host.string_param)
	__env_parse_int("HBTD_WARNTIME", &app_params.warntime.int_param)
	__env_parse_int("HBTD_ERRTIME", &app_params.errtime.int_param)
	__env_parse_int("HBTD_INTERVAL", &app_params.check_interval.int_param)
	__env_parse_int("HBTD_PORT", &app_params.port.int_param)
	__env_parse_string("HBTD_KV_URL", &app_params.kv_url.string_param)
	__env_parse_string("HBTD_SM_URL", &app_params.statemgr_url.string_param)
	__env_parse_int("HBTD_SM_RETRIES", &app_params.statemgr_retries.int_param)
	__env_parse_int("HBTD_SM_RETRIES", &app_params.statemgr_retries.int_param)
	__env_parse_int("HBTD_SM_TIMEOUT", &app_params.statemgr_timeout.int_param)
	__env_parse_int("HBTD_CLEAR_ON_GAP", &app_params.clear_on_gap.int_param)
}

/////////////////////////////////////////////////////////////////////////////
// Given a byte array of JSON data containing configurable parameter data,
// parse it and set the configurable params therein.  This function is told
// where the request comes from -- in some cases we want to set params in
// persistent storage, in some cases not; in some cases we want to report
// errors, in some cases not.
//
// parm_json(in): Byte array containing config params in JSON format.
// whence(in):    PARAM_START, PARAM_PATCH, PARAM_SYNC.
// errstr(out):   Generated errors encountered, for caller's use.
// Return:        0 on success, -1 on error.
/////////////////////////////////////////////////////////////////////////////

func parse_parm_json(parm_json []byte, whence int, errstr *string) int {
	var jdata inidata
	var tpd op_params

	bad := 0
	tpd = app_params

	bberr := json.Unmarshal(parm_json, &jdata)
	if bberr != nil {
		var v map[string]interface{}

		//The Unmarshal failed, find out which field(s) specifically failed.
		//There's no quick-n-dirty way to do this so we'll just bulldoze
		//through each field and verify it.

		errb := json.Unmarshal(parm_json, &v)
		if errb != nil {
			hbtdPrintln("Unmarshal into map[string]interface{} didn't work:", errb)
			*errstr = "Invalid JSON data type"
		} else {
			//Figure out what field(s) == bad and report them.  For now, they're
			//all strings.

			mtype := reflect.TypeOf(jdata)
			for i := 0; i < mtype.NumField(); i++ {
				nm := mtype.Field(i).Name
				if v[nm] == nil {
					continue
				}
				_, ok := v[nm].(string) //for now everything is strings.
				if !ok {
					*errstr += fmt.Sprintf("Invalid data type in %s field. ", nm)
					break
				}
			}
		}
		return -1
	}

	if jdata.Debug != "" {
		xx, err := strconv.ParseUint(jdata.Debug, 0, 32)
		if err != nil {
			*errstr += fmt.Sprintf("Parameter '%s' with illegal value '%s'; ",
				app_params.debug_level.name, jdata.Debug)
			bad = -1
		} else {
			tpd.debug_level.int_param = int(xx)
		}
	}

	if jdata.Nosm != "" {
		lcut := strings.ToLower(jdata.Nosm)
		if (lcut == "0") || (lcut == "no") || (lcut == "off") || (lcut == "false") {
			tpd.nosm.int_param = 0
		} else if (lcut == "1") || (lcut == "yes") || (lcut == "on") || (lcut == "true") {
			tpd.nosm.int_param = 1
		} else {
			*errstr += fmt.Sprintf("Parameter '%s' with illegal value '%s'; ",
				app_params.nosm.name, jdata.Nosm)
			bad = -1
		}
	}

	if jdata.Use_telemetry != "" {
		lcut := strings.ToLower(jdata.Use_telemetry)
		if (lcut == "0") || (lcut == "no") || (lcut == "off") || (lcut == "false") {
			tpd.use_telemetry.int_param = 0
		} else if (lcut == "1") || (lcut == "yes") || (lcut == "on") || (lcut == "true") {
			tpd.use_telemetry.int_param = 1
		} else {
			*errstr += fmt.Sprintf("Parameter '%s' with unknown value '%s'; ",
				app_params.use_telemetry.name, jdata.Use_telemetry)
			bad = -1
		}
	}

	if jdata.Telemetry_host != "" {
		_, _, _, err := get_telemetry_host(jdata.Telemetry_host)
		if err != nil {
			*errstr += fmt.Sprintf("Parameter '%s' with invalid format '%s'; ",
				app_params.telemetry_host.name, jdata.Telemetry_host)
			bad = -1
		} else {
			tpd.telemetry_host.string_param = jdata.Telemetry_host
		}
	}

	if jdata.Warntime != "" {
		xx, err := strconv.ParseUint(jdata.Warntime, 0, 32)
		if err != nil {
			*errstr += fmt.Sprintf("Parameter '%s' with illegal value '%s'; ",
				app_params.warntime.name, jdata.Warntime)
			bad = -1
		} else {
			tpd.warntime.int_param = int(xx)
		}
	}

	if jdata.Errtime != "" {
		xx, err := strconv.ParseUint(jdata.Errtime, 0, 32)
		if err != nil {
			*errstr += fmt.Sprintf("Parameter '%s' with illegal value '%s'; ",
				app_params.errtime.name, jdata.Errtime)
			bad = -1
		} else {
			tpd.errtime.int_param = int(xx)
		}
	}

	if jdata.Interval != "" {
		xx, err := strconv.ParseUint(jdata.Interval, 0, 32)
		if err != nil {
			*errstr += fmt.Sprintf("Parameter '%s' with illegal value '%s'; ",
				app_params.check_interval.name, jdata.Interval)
			bad = -1
		} else {
			tpd.check_interval.int_param = int(xx)
		}
	}

	if jdata.Kv_url != "" {
		tpd.kv_url.string_param = jdata.Kv_url
	}

	// Port is only settable at startup.  Don't allow any
	// changes; warn if patch is attempted.

	if jdata.Port != "" {
		if whence == PARAM_PATCH {
			*errstr += fmt.Sprintf("Parameter '%s' can't be changed in PATCH operation; ",
				app_params.port.name)
			bad = -1
		}
	}

	if jdata.Sm_url != "" {
		tpd.statemgr_url.string_param = jdata.Sm_url
	}

	if jdata.Sm_timeout != "" {
		xx, err := strconv.ParseUint(jdata.Sm_timeout, 0, 32)
		if err != nil {
			*errstr += fmt.Sprintf("Parameter '%s' with illegal value '%s'; ",
				app_params.statemgr_timeout.name, jdata.Sm_timeout)
			bad = -1
		} else {
			tpd.statemgr_timeout.int_param = int(xx)
		}
	}

	if jdata.Sm_retries != "" {
		xx, err := strconv.ParseUint(jdata.Sm_retries, 0, 32)
		if err != nil {
			*errstr += fmt.Sprintf("Parameter '%s' with illegal value '%s'; ",
				app_params.statemgr_retries.name, jdata.Sm_retries)
			bad = -1
		} else {
			tpd.statemgr_retries.int_param = int(xx)
		}
	}

	if bad == 0 {
		//Apply the previous app_params (tpd) + new stuff to app_params.
		//If badness happened, don't apply any of the new stuff.
		app_params = tpd
		server_url_port = tpd.port.string_param
	}

	return bad
}

/////////////////////////////////////////////////////////////////////////////
// Thread to connect to telemetry bus.  Retry until successful, or stop
// if we turn telemetry bus usage off before we actuall connect.
//
// Args, return: None
/////////////////////////////////////////////////////////////////////////////

func telebusConnect() {
	for {
		if app_params.use_telemetry.int_param == 0 {
			if msgbusHandle != nil {
				tbMutex.Lock()
				msgbusHandle.Disconnect()
				msgbusHandle = nil
				tbMutex.Unlock()
				hbtdPrintf("Disconnected from telemetry bus.\n")
			}
		} else {
			if msgbusHandle == nil {
				host, port, topic, terr := get_telemetry_host(app_params.telemetry_host.string_param)
				if terr != nil {
					hbtdPrintln("ERROR: telemetry host is not set or is invalid:", terr)
				} else {
					if app_params.debug_level.int_param > 0 {
						hbtdPrintf("Connecting to telemetry host: '%s:%d:%s'\n",
							host, port, topic)
					}
					msgbusConfig.Host = host
					msgbusConfig.Port = port
					msgbusConfig.Topic = topic
					msgbusConfig.ConnectRetries = 1
					tbMutex.Lock()
					msgbusHandle, terr = msgbus.Connect(msgbusConfig)
					if terr != nil {
						hbtdPrintln("ERROR connecting to telemetry bus, retrying...:",
							terr)
						msgbusHandle = nil
					} else {
						hbtdPrintf("Connected to Telemetry Bus.\n")
					}
					tbMutex.Unlock()
				}
			}
		}

		time.Sleep(5 * time.Second)
	}
}

/////////////////////////////////////////////////////////////////////////////
// Open up a connection to the KV store and initialize.  Ugh, this is
// complex.  In Dockerfile, you can't create env vars using other env vars.
// And, we get ETCD_HOST and ETCD_PORT from the ETCD operator as env vars.
//
// So, what we'll do is use KV_URL env var in Dockerfile.  If it's not
// empty, use it as is.  If it's empty then we'll check our env vars for
// ETCD_HOST and ETCD_PORT and create a URL from that.  If those aren't
// set, we'll fail.
//
// Args, return: None
/////////////////////////////////////////////////////////////////////////////

func openKV() {
	var kverr error

	if app_params.kv_url.string_param == "" {
		eh := os.Getenv("ETCD_HOST")
		ep := os.Getenv("ETCD_PORT")
		if (eh != "") && (ep != "") {
			app_params.kv_url.string_param = fmt.Sprintf("http://%s:%s", eh, ep)
			fmt.Printf("INFO: Setting KV URL from ETCD_HOST and ETCD_PORT (%s)\n",
				app_params.kv_url.string_param)
		} else {
			//This is a hard fail.  We could just fall back to "mem:" but
			//that will fail in very strange ways in multi-instance mode.
			//Hard fail KV connectivity.

			log.Printf("ERROR: KV URL is not set (no ETCD_HOST/ETCD_PORT and no KV_URL)!  Can't continue.\n")
			for {
				time.Sleep(1000 * time.Second)
			}
		}
	}

	// Try to open connection to ETCD.  This service is worthless until
	// this succeeds, so try forever.  Liveness and readiness probes will
	// fail until it works, which is the Kubernetes Way(TM).

	ix := 1
	for {
		kvHandle, kverr = hmetcd.Open(app_params.kv_url.string_param, "")
		if kverr != nil {
			hbtdPrintf("ERROR opening connection to ETCD (%s) (attempt %d): %v",
				app_params.kv_url.string_param,ix,kverr)
		} else {
			hbtdPrintf("ETCD connection succeeded.\n")
			break
		}
		ix ++
		time.Sleep(5 * time.Second)
	}

	//Wait for ETCD connectivity to be OK.  Again, try forever.

	ix = 1
	for {
		kerr := kvHandle.Store(HBTD_HEALTH_KEY, HBTD_HEALTH_OK)
		if kerr == nil {
			hbtdPrintf("K/V health check succeeded.\n")
			break
		}
		hbtdPrintf("ERROR: K/V health key store failed, attempt %d.",ix)
		time.Sleep(5 * time.Second)
		ix ++
	}
}

// Create a unique instance key to be used to indicate we are running.  This
// will be used to create a temp key which will live only as long as this
// instance of the services does.  This provides a way for any given instance
// to know, at startup, if there have been 0 instance running for some amount
// of time, which is critical to prevent false HB-stop notifications.
// The key will consist of the life key prefix plus a random number string.

func createInstanceKey() string {
	rand.Seed(time.Now().UnixNano())
	ikey := fmt.Sprintf("%s%d",HBTD_LIFE_KEY_PRE,rand.Int31())
	return ikey
}

// Check to see if there are any HBTD life keys.  If there are none, that means
// we are the first instance to run.  This can mean that there were >=1 inst
// running at some time in the past, and if so, there is HB info that is stale.
// That info has to be deleted and re-discovered.

func checkLifeKeys() {
	lstart := HBTD_LIFE_KEY_PRE+fmt.Sprintf("%d",HBTD_LIFE_KEY_START)
	lend   := HBTD_LIFE_KEY_PRE+fmt.Sprintf("%d",HBTD_LIFE_KEY_END)

	kvlist,kverr := kvHandle.GetRange(lstart,lend)
	if (kverr != nil) {
		hbtdPrintf("ERROR: Can't retrieve life keys: %v\nAssuming the worst,deleting all HB key info.")
		staleKeys = true
	} else {
		if (len(kvlist) == 0) {
			hbtdPrintf("INFO: No life keys found, HB key cleanup set to: %d.",
				app_params.clear_on_gap.int_param)
			staleKeys = true
		}
	}

	if (staleKeys && (app_params.clear_on_gap.int_param != 0)) {
		hbkeys, hbkerr := kvHandle.GetRange(HB_KEYRANGE_START,HB_KEYRANGE_END)
		if (hbkerr != nil) {
			hbtdPrintf("ERROR: Trying to delete old HB keys, can't fetch any keys.")
			return
		}
		for _,kv := range hbkeys {
			err := kvHandle.Delete(kv.Key)
			if (err != nil) {
				hbtdPrintf("ERROR: Problem trying to delete old HB key %s': %v",
					kv.Key,err)
			}
		}
		hbtdPrintf("INFO: old HB keys cleared.")
	}
}

/////////////////////////////////////////////////////////////////////////////
// Print current application parameters.
//
// Args, Return: None.
/////////////////////////////////////////////////////////////////////////////

func printParams() {
	hbtdPrintf("debug_level    %d\n", app_params.debug_level.int_param)
	hbtdPrintf("nosm           %d\n", app_params.nosm.int_param)
	hbtdPrintf("use_telemetry  %d\n", app_params.use_telemetry.int_param)
	hbtdPrintf("telemetry_host %s\n", app_params.telemetry_host.string_param)
	hbtdPrintf("warntime       %d\n", app_params.warntime.int_param)
	hbtdPrintf("errtime        %d\n", app_params.errtime.int_param)
	hbtdPrintf("port           %s\n", app_params.port.string_param)
	hbtdPrintf("kv_url         %s\n", app_params.kv_url.string_param)
	hbtdPrintf("interval       %d\n", app_params.check_interval.int_param)
	hbtdPrintf("sm_url         %s\n", app_params.statemgr_url.string_param)
	hbtdPrintf("sm_timeout     %d\n", app_params.statemgr_timeout.int_param)
	hbtdPrintf("sm_retries     %d\n", app_params.statemgr_retries.int_param)
}

/////////////////////////////////////////////////////////////////////////////
// Entry point.
/////////////////////////////////////////////////////////////////////////////

func main() {
	var err error

	hbtdPrintf("Cray Node Heartbeat Monitor/Tracker started.\n")

	serviceName, err = base.GetServiceInstanceName()
	if err != nil {
		log.Printf("ERROR: can't get service/host name!  Using 'HBTD'.\n")
		serviceName = "HBTD"
	}
	log.Printf("Service name: '%s'",serviceName)

	initAppParams()

	//Gather ENV vars.  These are the first level of parameters.

	parse_env_vars()

	//Parse cmdline params if any.  These override .ini params and env vars

	parse_cmd_line()

	if app_params.debug_level.int_param > 0 {
		printParams()
	}

	// Set up http transport for outbound stuff

	htrans.transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	htrans.client = &http.Client{Transport: htrans.transport,
		Timeout: (time.Duration(app_params.statemgr_timeout.int_param) *
			time.Second),
	}

	//Fire up HSM readiness checker

	go checkHSM()

	//Wait until HSM is ready.

	waitForHSM()

	// KV store connection

	openKV()

	//Generate a unique instance key and check for HBTD life keys.  If none, 
	//delete stale HB data in KV store.

	instanceKey := createInstanceKey()
	checkLifeKeys()

	// Write our instance-specific life key

	go func() {
		for {
			err := kvHandle.TempKey(instanceKey)
			if err != nil {
				hbtdPrintf("ERROR: Can't create life key '%s', retrying...",
					instanceKey)
			} else {
				hbtdPrintf("Life key '%s' created.",instanceKey)
				break
			}
			time.Sleep(2 * time.Second)
		}
	}()

	//Start the thread for handling state mgr messaging

	go send_sm_req()

	// ** Used for testing only **
	// Sync up to the wall clock so we start at XX:00:00
	//for {
	//	sec := time.Now().Second()
	//	if (sec == 0) {
	//		break
	//	}
	//	time.Sleep(10 * time.Millisecond)
	//}
	// **

	// Start the heartbeat and param checker timers

	rearm_hbcheck_timer()

	//Fire up telemetry bus connect thread

	go telebusConnect()
	go telemetry_handler()

	hbtdPrintf("Listening on port %s\n", server_url_port)

	// Fire up the web service and enter the server loop.

	routes := generateRoutes()
	router := newRouter(routes)
	srv := &http.Server{Addr: ":"+server_url_port, Handler: router,}

	//Set up signal handling for graceful kill

	c := make(chan os.Signal,1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	idleConnsClosed := make(chan struct{})

	go func() {
		<-c
		Running = false

		//Gracefully shutdown the HTTP server
		lerr := srv.Shutdown(context.Background())
		if (lerr != nil) {
			log.Printf("ERROR: HTTP server shutdown error: %v",lerr)
		}
		close(idleConnsClosed)
	}()

	log.Printf("INFO: Starting up HTTP server.")
	srvErr := srv.ListenAndServe()
	if (srvErr != http.ErrServerClosed) {
		log.Printf("FATAL: HTTP server ListenandServe failed: %v",srvErr)
	}

	log.Printf("INFO: Server shutdown, waiting for idle connections to close...")
	<-idleConnsClosed
	log.Printf("INFO: Done.  Exiting.")

	os.Exit(0)
}

