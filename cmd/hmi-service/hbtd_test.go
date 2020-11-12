// Copyright 2018-2020 Cray Inc.

package main

import (
    "testing"
    "encoding/json"
    "strconv"
    "os"
    "strings"
)

type inidata_plus struct {
    params inidata
    env_var string
    jstr string
}

// INI file data test cases

var ini_set = []inidata_plus { 
    {
        jstr: "{\"Debug\":\"1\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_DEBUG=1",
        params: inidata {
            Debug: "1",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"1\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_NOSM=1",
        params: inidata {
            Debug: "0",
            Nosm: "1",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"1\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_USE_TELEMETRY=1",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "1",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"localhost:9092:heartbeat_notifications\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_TELEMETRY_HOST=localhost:9092:heartbeat_notifications",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "localhost:9092:heartbeat_notifications",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"5\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_WARNTIME=5",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "5",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"6\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_ERRTIME=6",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "6",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"https://localhost:1234/kvstore\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_KV_URL=https://localhost:1234/kvstore",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "https://localhost:1234/kvstore",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"12\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_INTERVAL=12",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "12",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"http://a.b.c:8989/hmi/v1\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_SM_URL=http://a.b.c:8989/hmi/v1",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "http://a.b.c:8989/hmi/v1",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"5\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_SM_TIMEOUT=5",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "5",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"6\"}",
        env_var: "HBTD_SM_RETRIES=6",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "6",
        },
    },
}

var fail_set = []inidata_plus {
    {
        jstr: "{\"Debug\":\"x\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_DEBUG=x",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":0,\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_DEBUG=x",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"x\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_NOSM=x",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"x\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_USE_TELEMETRY=x",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"x\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_WARNTIME=x",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"x\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_ERRTIME=x",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"x\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_INTERVAL=x",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"x\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_SM_TIMEOUT=x",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"x\"}",
        env_var: "HBTD_SM_RETRIES=x",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
    {
        jstr: "{\"Debug\":\"0\",\"Nosm\":\"0\",\"Use_telemetry\":\"0\",\"Telemetry_host\":\"\",\"Warntime\":\"0\",\"Errtime\":\"0\",\"Port\":\"1234\",\"Kv_url\":\"\",\"Interval\":\"0\",\"Sm_url\":\"\",\"Sm_timeout\":\"0\",\"Sm_retries\":\"0\"}",
        env_var: "HBTD_PORT=x",
        params: inidata {
            Debug: "0",
            Nosm: "0",
            Use_telemetry: "0",
            Telemetry_host: "",
            Warntime: "0",
            Errtime: "0",
            Port: "",
            Kv_url: "",
            Interval: "0",
            Sm_url: "",
            Sm_timeout: "0",
            Sm_retries: "0",
        },
    },
}

var printHelpOutput string = `Usage: ./hbtd [options]
  --help                      Help text.
  --debug=num                 Debug level.  (Default: 0)
  --use_telemetry=yes|no      Inject notifications into message.
                              bus. (Default: yes)
  --telemetry_host=h:p:t      Hostname:port:topic of telemetry service
  --warntime=secs             Seconds before sending a warning of
                              node heartbeat failure.  
                              (Default: 10 seconds)
  --errtime=secs              Seconds before sending an error of
                              node heartbeat failure.  
                              (Default: 30 seconds)
  --interval=secs             Heartbeat check interval.
                              (Default: 5 seconds)
  --port=num                  HTTPS port to listen on.  (Default: 28500)
  --kv_url=url                Key-Value service 'base' URL..  (Default: https://localhost:2379)
  --sm_url=url                State Manager 'base' URL.  (Default: http://localhost:27779/hsm/v1)
  --sm_retries=num            Number of State Manager access retries. (Default: 3)
  --sm_timeout=secs           State Manager access timeout. (Default: 10)
  --nosm                      Don't contact State Manager (for testing).
`

var printParamsOutput = `debug_level    0
nosm           0
use_telemetry  1
telemetry_host 
warntime       10
errtime        30
port           28500
kv_url         https://localhost:2379
interval       5
sm_url         http://localhost:27779/hsm/v1
sm_timeout     10
sm_retries     3
`

// Zero's out the global app_params data

func zero_app_params() {
    app_params.debug_level = app_param{"",0,""}
    app_params.nosm = app_param{"",0,""}
    app_params.use_telemetry = app_param{"",0,""}
    app_params.telemetry_host = app_param{"",0,""}
    app_params.warntime = app_param{"",0,""}
    app_params.errtime = app_param{"",0,""}
    app_params.port = app_param{"",0,""}
    app_params.kv_url = app_param{"",0,""}
    app_params.check_interval = app_param{"",0,""}
    app_params.statemgr_url = app_param{"",0,""}
    app_params.statemgr_timeout = app_param{"",0,""}
    app_params.statemgr_retries = app_param{"",0,""}
}

// Compare an app parameter structure against the global app params.
//
// t(in):  Test framework
// opp(in): App parameters from a parsed ini file to compare against the 
//          global one.
// Return:  0 on success, -1 on error

func compare_params(t *testing.T, opp inidata_plus) int {
    var ok int = 0
    var ival int
    var err error

    ival,err = strconv.Atoi(opp.params.Debug)
    if (err != nil) {
        t.Error("Error converting 'Debug' parameter ",opp.params.Debug," to integer.")
    }
    if (ival != app_params.debug_level.int_param) {
        t.Error("Mismatch debug level: expected/got:",
            ival,app_params.debug_level.int_param)
        ok = -1
    }
    ival,err = strconv.Atoi(opp.params.Nosm)
    if (err != nil) {
        t.Error("Error converting 'Nosm' parameter ",opp.params.Nosm," to integer.")
    }
    if (ival != app_params.nosm.int_param) {
        t.Error("Mismatch nosm: expected/got:",
            ival,app_params.nosm.int_param)
        ok = -1
    }
    ival,err = strconv.Atoi(opp.params.Use_telemetry)
    if (err != nil) {
        t.Error("Error converting 'Use_telemetry' parameter",opp.params.Use_telemetry," to integer.")
    }
    if (ival != app_params.use_telemetry.int_param) {
        t.Error("Mismatch use_telemetry level: expected/got:",
            ival,app_params.use_telemetry.int_param)
        ok = -1
    }
    if (opp.params.Telemetry_host != app_params.telemetry_host.string_param) {
        t.Error("Mismatch 'Telemetry_host' parameter: expected/got:",
            opp.params.Telemetry_host,app_params.telemetry_host.string_param)
        ok = -1
    }
    ival,err = strconv.Atoi(opp.params.Warntime)
    if (err != nil) {
        t.Error("Error converting 'Warntime' parameter ",opp.params.Warntime," to integer.")
    }
    if (ival != app_params.warntime.int_param) {
        t.Error("Mismatch warntime level: expected/got:",
            ival,app_params.warntime.int_param)
        ok = -1
    }
    ival,err = strconv.Atoi(opp.params.Errtime)
    if (err != nil) {
        t.Error("Error converting 'Errtime' parameter ",opp.params.Errtime," to integer.")
    }
    if (ival != app_params.errtime.int_param) {
        t.Error("Mismatch errtime level: expected/got:",
            ival,app_params.errtime.int_param)
        ok = -1
    }
    if (opp.params.Port != app_params.port.string_param) {
        t.Error("Mismatch port level: expected/got:",
            opp.params.Port,app_params.port.string_param)
        ok = -1
    }
    if (opp.params.Kv_url != app_params.kv_url.string_param) {
        t.Error("Mismatch KV URL: expected/got:",
            opp.params.Kv_url,app_params.kv_url.string_param)
        ok = -1
    }
    ival,err = strconv.Atoi(opp.params.Interval)
    if (err != nil) {
        t.Error("Error converting 'Interval' parameter",opp.params.Interval," to integer.")
    }
    if (ival != app_params.check_interval.int_param) {
        t.Error("Mismatch check_interval level: expected/got:",
            ival,app_params.check_interval.int_param)
        ok = -1
    }
    if (opp.params.Sm_url != app_params.statemgr_url.string_param) {
        t.Error("Mismatch statemgr_url level: expected/got:",
            opp.params.Sm_url,app_params.statemgr_url.string_param)
        ok = -1
    }
    ival,err = strconv.Atoi(opp.params.Sm_timeout)
    if (err != nil) {
        t.Error("Error converting 'Sm_timeout' parameter ",opp.params.Sm_timeout," to integer.")
    }
    if (ival != app_params.statemgr_timeout.int_param) {
        t.Error("Mismatch statemgr_timeout: expected/got:",
            ival,app_params.statemgr_timeout.int_param)
        ok = -1
    }
    ival,err = strconv.Atoi(opp.params.Sm_retries)
    if (err != nil) {
        t.Error("Error converting 'Sm_retries' parameter ",opp.params.Sm_retries," to integer.")
    }
    if (ival != app_params.statemgr_retries.int_param) {
        t.Error("Mismatch statemgr_retries: expected/got:",
            ival,app_params.statemgr_retries.int_param)
        ok = -1
    }
    return ok
}

// Test entry point for parse_parm_json()

func TestParse_parm_json(t *testing.T) {

    var rc int
    var errstr string

    hbtdPrintf = testPrintf
    hbtdPrintln = testPrintln

    t.Logf("** RUNNING PARAM PARSE TEST **\n")

    //Test happy path

    for testIX := 0; testIX < len(ini_set); testIX++ {
        t.Logf("Executing happy test %d of %d...\n",testIX+1,len(ini_set))
        zero_app_params()

        //Marshall
        ba, err := json.Marshal(ini_set[testIX].params)
        if (err != nil) {
            t.Error("ERROR marshalling test data index ",testIX)
        }

        //render into app_params

        if (parse_parm_json(ba,PARAM_START,&errstr) != 0) {
            t.Error("ERROR parsing parameter JSON string:",errstr)
        }

        rc = compare_params(t,ini_set[testIX])
        if (rc != 0) {
            t.Error("ERROR comparing expected param parsing results.")
        }
    }

    //Test fail path

    for failIX := 0; failIX < len(fail_set); failIX++ {
        t.Logf("Executing fail test %d of %d...\n",failIX+1,len(fail_set))
        zero_app_params()

        if (parse_parm_json([]byte(fail_set[failIX].jstr),PARAM_PATCH,&errstr) == 0) {
            t.Error("ERROR, fail case didn't fail to parse JSON:",fail_set[failIX].jstr)
        }
    }
        
    t.Logf("  ==> FINISHED PARAM PARSING\n")
}

// Test entry point for gen_cur_param_json()

func TestGen_cur_param_json(t *testing.T) {

    var errstr string

    hbtdPrintf = testPrintf
    hbtdPrintln = testPrintln

    t.Logf("** RUNNING PARAM JSON GENERATOR TEST **\n")


    for testIX := 0; testIX < len(ini_set); testIX++ {
        t.Logf("Executing test %d of %d...\n",testIX+1,len(ini_set))
        zero_app_params()

        //Marshall
        ba, err := json.Marshal(ini_set[testIX].params)
        if (err != nil) {
            t.Error("ERROR marshalling test data index ",testIX)
        }

        //render into app_params

        if (parse_parm_json(ba,PARAM_START,&errstr) != 0) {
            t.Error("ERROR parsing parameter JSON string:",errstr)
        }

        var ba2 []byte
        if (gen_cur_param_json(&ba2) != 0) {
            t.Error("ERROR generating current parameter JSON string.")
        }

        if (ini_set[testIX].jstr != string(ba2)) {
            t.Error("ERROR comparing expected param parsing, exp: ",
                ini_set[testIX].jstr," got: ",string(ba2))
        }
    }
        
    t.Logf("  ==> FINISHED PARAM JSON GENERATOR TEST\n")
}

// Test entry point for parse_env_vars()

func TestParse_env_vars(t *testing.T) {

    var rc int

    hbtdPrintf = testPrintf
    hbtdPrintln = testPrintln

    t.Logf("** RUNNING ENV PARAM PARSE TEST **\n")

    //Happy path

    for testIX := 0; testIX < len(ini_set); testIX++ {
        t.Logf("Executing happy test %d of %d...\n",testIX+1,len(ini_set))
        zero_app_params()

        //Set env var
        vvals := strings.Split(ini_set[testIX].env_var,"=")
        os.Setenv(vvals[0],vvals[1])

        //Parse env var set

        parse_env_vars()
        os.Unsetenv(vvals[0])

        rc = compare_params(t,ini_set[testIX])
        if (rc != 0) {
            t.Error("ERROR comparing expected param parsing results.")
        }
    }

    // Failure tests

    for failIX := 0; failIX < len(fail_set); failIX++ {
        t.Logf("Executing fail test %d of %d...\n",failIX+1,len(fail_set))
        zero_app_params()

        //Set env var
        vvals := strings.Split(fail_set[failIX].env_var,"=")
        os.Setenv(vvals[0],vvals[1])

        //Parse env var set

        parse_env_vars()
        os.Unsetenv(vvals[0])

        rc = compare_params(t,fail_set[failIX])
        if (rc != 0) {
            t.Error("ERROR comparing expected param parsing results.")
        }
    }
       
    t.Logf("  ==> FINISHED ENV PARAM PARSE TEST\n")
}

func TestPrintHelp(t *testing.T) {
    
    hbtdPrintf = testPrintf
    hbtdPrintln = testPrintln

    testPrintClear()
    printHelp()

    std_toks := strings.Split(testStdout,"\n")
    exp_toks := strings.Split(printHelpOutput,"\n")

    if (len(std_toks) < len(exp_toks)) {
        t.Fatalf("ERROR, mismatch in help text strings count, exp: %d, actual: %d\n",
            len(exp_toks),len(std_toks))
    }

    //Skip the first string, as it has the app name, which due to the way
    //testing works, won't match.

    for ix := 1; ix < len(exp_toks); ix ++ {
        sline := strings.TrimSpace(std_toks[ix])
        eline := strings.TrimSpace(exp_toks[ix])

        if (sline != eline) {
            t.Errorf("ERROR Mismatch help text, exp: .%s., got: .%s.\n",
                eline,sline)
        }
    }
}

func TestGet_telemetry_host(t *testing.T) {
    hbtdPrintf = testPrintf
    hbtdPrintln = testPrintln

    testPrintClear()

    var t1,t3 string
    var t2 int
    var err error

    t1,t2,t3,err = get_telemetry_host("aaa:111:ccc")
    if (err != nil) {
        t.Error("ERROR splitting telemetry host:",err)
    }

    if (t1 != "aaa") {
        t.Errorf("ERROR splitting telemetry host, expected 'aaa', got '%s'\n",
            t1)
    }
    if (t2 != 111) {
        t.Errorf("ERROR splitting telemetry host, expected '111', got '%d'\n",
            t2)
    }
    if (t3 != "ccc") {
        t.Errorf("ERROR splitting telemetry host, expected 'ccc', got '%s'\n",
            t3)
    }

    //Test with string t2

    t1,t2,t3,err = get_telemetry_host("ddd:eee:fff")
    if (err == nil) {
        t.Errorf("ERROR, string port didn't fail.\n")
    }

    //Test with too few toks

    t1,t2,t3,err = get_telemetry_host("www:222")
    if (err == nil) {
        t.Errorf("ERROR, too few tokens, but didn't fail to parse.\n")
    }
}

func TestParse_cmd_line(t *testing.T) {
    hbtdPrintf = testPrintf
    hbtdPrintln = testPrintln

    testPrintClear()
    initAppParams()

    os.Args = []string{"app", "--debug=1", "--kv_url=a.b.c.d", "--nosm",
                       "--port=1234", "--warntime=5", "--errtime=10",
                       "--sm_retries=12", "--sm_timeout=34",
                       "--sm_url=e.f.g.h", "--telemetry_host=aaaa:1234:bbbb",
                       "--interval=12", "--use_telemetry=1"}

    parse_cmd_line()

    if (app_params.debug_level.int_param != 1) {
        t.Errorf("ERROR, debug_level incorrect, expected 1, got %d\n",
            app_params.debug_level.int_param)
    }
    if (app_params.kv_url.string_param != "a.b.c.d") {
        t.Errorf("ERROR, kv_url incorrect, expected 'a.b.c.d', got '%s'\n",
            app_params.kv_url.string_param)
    }
    if (app_params.nosm.int_param != 1) {
        t.Errorf("ERROR, nosm incorrect, expected 1, got %d\n",
            app_params.nosm.int_param)
    }
    if (app_params.port.string_param != "1234") {
        t.Errorf("ERROR, port incorrect, expected '1234', got '%s'\n",
            app_params.port.string_param)
    }
    if (app_params.warntime.int_param != 5) {
        t.Errorf("ERROR, warntime incorrect, expected 5, got %d\n",
            app_params.warntime.int_param)
    }
    if (app_params.errtime.int_param != 10) {
        t.Errorf("ERROR, errtime incorrect, expected 10, got %d\n",
            app_params.errtime.int_param)
    }
    if (app_params.statemgr_retries.int_param != 12) {
        t.Errorf("ERROR, statemgr_retries incorrect, expected 12, got %d\n",
            app_params.statemgr_retries.int_param)
    }
    if (app_params.statemgr_timeout.int_param != 34) {
        t.Errorf("ERROR, statemgr_timeout incorrect, expected 34, got %d\n",
            app_params.statemgr_timeout.int_param)
    }
    if (app_params.statemgr_url.string_param != "e.f.g.h") {
        t.Errorf("ERROR, statemgr_url incorrect, expected 'e.f.g.h', got '%s'\n",
            app_params.statemgr_url.string_param)
    }
    if (app_params.telemetry_host.string_param != "aaaa:1234:bbbb") {
        t.Errorf("ERROR, telemetry_host incorrect, expected 'aaaa:1234:bbbb', got '%s'\n",
            app_params.telemetry_host.string_param)
    }
    if (app_params.use_telemetry.int_param != 1) {
        t.Errorf("ERROR, use_telemetry incorrect, expected 1, got %d\n",
            app_params.use_telemetry.int_param)
    }

    //Tests some error and harder to reach cases.

    unintm1 := UNINT-1
    tvars := op_params{debug_level:      app_param{name:"",int_param:unintm1,string_param:"",},
                       nosm:             app_param{name:"",int_param:0,string_param:"",},
                       use_telemetry:    app_param{name:"",int_param:0,string_param:"xyzzy",},
                       telemetry_host:   app_param{name:"",int_param:0,string_param:"aaa:222",},
                       warntime:         app_param{name:"",int_param:unintm1,string_param:"",},
                       errtime:          app_param{name:"",int_param:unintm1,string_param:"",},
                       check_interval:   app_param{name:"",int_param:unintm1,string_param:"",},
                       port:             app_param{name:"",int_param:0,string_param:"xyzzy",},
                       kv_url:           app_param{name:"",int_param:0,string_param:UNSTR,},
                       statemgr_url:     app_param{name:"",int_param:0,string_param:UNSTR,},
                       statemgr_retries: app_param{name:"",int_param:unintm1,string_param:"",},
                       statemgr_timeout: app_param{name:"",int_param:unintm1,string_param:"",},
    }

    initAppParams()
    ap_thost := app_params.telemetry_host.string_param
    ap_kv_url := app_params.kv_url.string_param
    ap_sm_url := app_params.statemgr_url.string_param

    parse_cmdline_params(tvars)
    if (app_params.debug_level.int_param != 0) {
        t.Errorf("ERROR, parse of debug_level=%d didn't correct to 0.\n",unintm1)
    }
    if (app_params.use_telemetry.int_param != 0) {
        t.Errorf("ERROR, parse of use_telemetry=0 didn't correct to 0.\n")
    }
    tvars.use_telemetry.string_param = "no"
    parse_cmdline_params(tvars)
    if (app_params.use_telemetry.int_param != 0) {
        t.Errorf("ERROR, parse of use_telemetry=0 didn't parse to 0.\n")
    }
    if (app_params.telemetry_host.string_param != ap_thost) {
        t.Errorf("ERROR, bad parse of telemetry_host didn't fail.\n")
    }
    if (app_params.warntime.int_param != 0) {
        t.Errorf("ERROR, parse of warntime=%d didn't correct to 0.\n",unintm1)
    }
    if (app_params.errtime.int_param != 0) {
        t.Errorf("ERROR, parse of errtime=%d didn't correct to 0.\n",unintm1)
    }
    if (app_params.check_interval.int_param != 0) {
        t.Errorf("ERROR, parse of check_interval=%d didn't correct to 0.\n",unintm1)
    }
    if (app_params.port.string_param != URL_PORT) {
        t.Errorf("ERROR, bad parse of port didn't fail.\n")
    }
    if (app_params.statemgr_retries.int_param != 1) {
        t.Errorf("ERROR, bad parse of statemgr_retries=%d didn't correct to 1.\n",unintm1)
    }
    if (app_params.statemgr_timeout.int_param != 1) {
        t.Errorf("ERROR, bad parse of statemgr_timeout=%d didn't correct to 1.\n",unintm1)
    }
    if (app_params.kv_url.string_param != ap_kv_url) {
        t.Errorf("ERROR, kv_url parse incorrectly didn't fail.\n")
    }
    if (app_params.statemgr_url.string_param != ap_sm_url) {
        t.Errorf("ERROR, statemgr_url parse incorrectly didn't fail.\n")
    }

    //Restore default app params.
    initAppParams()
}

func TestPrintParams(t *testing.T) {
    hbtdPrintf = testPrintf
    hbtdPrintln = testPrintln

    testPrintClear()

    initAppParams()
    printParams()

    std_toks := strings.Split(testStdout,"\n")
    exp_toks := strings.Split(printParamsOutput,"\n")

    if (len(std_toks) < len(exp_toks)) {
        t.Fatalf("ERROR, mismatch in help text strings count, exp: %d, actual: %d\n",
            len(exp_toks),len(std_toks))
    }

    //Skip the first string, as it has the app name, which due to the way
    //testing works, won't match.

    for ix := 1; ix < len(exp_toks); ix ++ {
        sline := strings.TrimSpace(std_toks[ix])
        eline := strings.TrimSpace(exp_toks[ix])

        if (sline != eline) {
            t.Errorf("ERROR Mismatch params text, exp: .%s., got: .%s.\n",
                eline,sline)
        }
    }

    //Restore params to defaults
    initAppParams()
}

func TestOpenKV(t *testing.T) {
	t.Log("Testing KV open with valid URL.")
	app_params.kv_url.string_param = "mem:"
	openKV()
	kvHandle = nil
}

func TestCheckLifeKeys(t *testing.T) {
	var err error
	t.Log("Testing checkLifeKeys() with no keys found.")

	app_params.kv_url.string_param = "mem:"
	openKV()
	app_params.debug_level.int_param = 3
	staleKeys = false
	checkLifeKeys()
	if (staleKeys == false) {
		t.Log("ERROR: checkLifeKeys() with no life keys present didn't flag stale keys.")
	}

	t.Log("Testing checkLifeKeys() with life key in place.")
	staleKeys = false
	ik := createInstanceKey()
	ik = HBTD_LIFE_KEY_PRE+"0"
	err = kvHandle.TempKey(ik)
	if err != nil {
		t.Errorf("Error setting TempKey: %v",err)
	}
	checkLifeKeys()
	if (staleKeys == true) {
		t.Log("ERROR: checkLifeKeys() with valid life key present flagged stale keys.")
	}
	kvHandle = nil
}

