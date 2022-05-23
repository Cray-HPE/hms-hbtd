// MIT License
//
// (C) Copyright [2022] Hewlett Packard Enterprise Development LP
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
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type ScnSubscribe struct {
	Subscriber     string   `json:"Subscriber"`               //[service@]xname (nodes) or 'hmnfd'
	Components     []string `json:"Components,omitempty"`     //SCN components (usually nodes)
	Url            string   `json:"Url"`                      //URL to send SCNs to
	States         []string `json:"States,omitempty"`         //Subscribe to these HW SCNs
	Enabled        *bool    `json:"Enabled,omitempty"`        //true==all enable/disable SCNs
	SoftwareStatus []string `json:"SoftwareStatus,omitempty"` //Subscribe to these SW SCNs
	//Flag bool               `json:"Flags,omitempty"`        //Subscribe to flag changes
	Roles []string `json:"Roles,omitempty"` //Subscribe to role changes
}

type hsmComponent struct {
	ID    string `json:"ID"`
	NID   string `json:"NID"`
	Type  string `json:"Type"`
	State string `json:"State"`
	Flag  string `json:"Flag"`
}

type hsmComponentList struct {
	Components []hsmComponent `json:"Components"`
}

type smjson_einfo struct {
	Id      string `json:"ID"`
	Message string `json:"Message"`
	Flag    string `json:"Flag"`
}

type bulkComponents struct {
	ComponentIDs []string `json:"ComponentIDs"`
	State        string   `json:"State"`
	Flag         string   `json:"Flag"`
	ExtendedInfo smjson_einfo
}

type grpMembers struct {
	IDS []string `jtag:"ids"`
}

type hsmGroup struct {
	Label          string     `jtag:"label"`
	Description    string     `jtag:"description"`
	Tags           []string   `jtag:"tags"`
	ExclusiveGroup string     `jtag:"exclusiveGroup"`
	Members        grpMembers `jtag:"members"`
}

// For State/Components

type hsmStateOnly struct {
	Flag  string `json:"Flag"`
	ID    string `json:"ID"`
	Type  string `json:"Type"`
	State string `json:"State"`
}

type hsmStateArray struct {
	Components []hsmStateOnly
}

type hsmGroupList []hsmGroup

var Groups hsmGroupList
var Components hsmComponentList

func doReady(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

/*
func hsmComponentsGet(w http.ResponseWriter, r *http.Request) {
	ba,baerr := json.Marshal(&Components)
	if (baerr != nil) {
		log.Printf("ERROR: problem marshalling component list: '%v'\n",baerr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type","application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(ba)
}
*/

func doServiceReady(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

var forceFail bool
var pauseSec = 0

func doBulkStateUpdate(w http.ResponseWriter, r *http.Request) {
	var bcomps bulkComponents

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: problem reading request body: '%v'\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = json.Unmarshal(body, &bcomps)
	if err != nil {
		log.Printf("ERROR: problem unmarshalling request body: '%v'\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	//Special case: If componentID[0] == "STOP", then all bulk state updates
	//from here on will fail.  If componentID[0] == "START" then go back
	//to normal.

	if bcomps.ComponentIDs[0] == "STOP" {
		forceFail = true
		log.Printf("Forcing future failure.")
	} else if bcomps.ComponentIDs[0] == "START" {
		forceFail = false
		log.Printf("Un-Forcing future failure.")
		w.WriteHeader(http.StatusOK)
		return
	} else if bcomps.ComponentIDs[0] == "PAUSE" {
		if len(bcomps.ComponentIDs) > 1 {
			pauseSec, _ = strconv.Atoi(bcomps.ComponentIDs[1])
			log.Printf("Setting up PAUSE for %d seconds.", pauseSec)
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	if forceFail {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if pauseSec > 0 {
		log.Printf("PAUSE for %d seconds.", pauseSec)
		time.Sleep(time.Duration(pauseSec) * time.Second)
		log.Printf("UN-PAUSE")
	}

	//copy(Components.Components,comps.Components)

	sort.Strings(bcomps.ComponentIDs)
	log.Printf("PATCH bulk components, comp list:")
	for ix, _ := range bcomps.ComponentIDs {
		log.Printf("    %s", bcomps.ComponentIDs[ix])
	}
	log.Printf("    State: %s", bcomps.State)
	log.Printf("    Flag:  %s", bcomps.Flag)
	log.Printf("    Xmsg:  %s", bcomps.ExtendedInfo.Message)

	w.WriteHeader(http.StatusOK)

}

func hsmComponentsPost(w http.ResponseWriter, r *http.Request) {
	//var comps hsmComponentList
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: problem reading request body: '%v'\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = json.Unmarshal(body, &Components)
	if err != nil {
		log.Printf("ERROR: problem unmarshalling request body: '%v'\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	//copy(Components.Components,comps.Components)

	log.Printf("POST components, comp list:")
	for ix, _ := range Components.Components {
		log.Printf("    %s", Components.Components[ix].ID)
	}

	w.WriteHeader(http.StatusOK)
}

func hsmComponentsXGet(w http.ResponseWriter, r *http.Request) {
	var rcomp hsmComponent
	vars := mux.Vars(r)
	xname := vars["xname"]

	for _, cmp := range Components.Components {
		if cmp.ID == xname {
			rcomp = cmp
			break
		}
	}

	if rcomp.ID == "" {
		log.Printf("ERROR: component '%s' not found.", xname)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ba, baerr := json.Marshal(&rcomp)
	if baerr != nil {
		log.Printf("ERROR: problem marshalling component %s: '%v'\n", xname, baerr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(ba)
}

func hsmComponentsXPatch(w http.ResponseWriter, r *http.Request) {
	var rcomp hsmComponent
	vars := mux.Vars(r)
	xname := vars["xname"]

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: problem reading request body: '%v'\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = json.Unmarshal(body, &rcomp)
	if err != nil {
		log.Printf("ERROR: problem unmarshalling request body: '%v'\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ok := false
	for ix, _ := range Components.Components {
		if Components.Components[ix].ID == xname {
			if rcomp.State != "" {
				Components.Components[ix].State = rcomp.State
			}
			if rcomp.Flag != "" {
				Components.Components[ix].Flag = rcomp.Flag
			}

			ok = true
			break
		}
	}

	if !ok {
		log.Printf("ERROR: component '%s' not found.", xname)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	envstr := os.Getenv("PATCHSTATUS")
	if envstr != "" {
		rval, _ := strconv.Atoi(envstr)
		log.Printf("Faking return value for State/Components/{xname}/StateData: %d",
			rval)
		w.WriteHeader(rval)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func hsmComponentsXPut(w http.ResponseWriter, r *http.Request) {
	var rcomp hsmComponent
	vars := mux.Vars(r)
	xname := vars["xname"]

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Printf("ERROR: problem reading request body: '%v'\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	err = json.Unmarshal(body, &rcomp)
	if err != nil {
		log.Printf("ERROR: problem unmarshalling request body: '%v'\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ok := false
	for ix, _ := range Components.Components {
		if Components.Components[ix].ID == xname {
			log.Printf("PUT Matched component '%s', updating", xname)
			if rcomp.State != "" {
				Components.Components[ix].State = rcomp.State
			}
			if rcomp.Flag != "" {
				Components.Components[ix].Flag = rcomp.Flag
			}

			ok = true
			break
		}
	}

	if !ok {
		log.Printf("PUT New component '%s'", xname)
		//Append
		Components.Components = append(Components.Components, rcomp)
	}

	w.WriteHeader(http.StatusOK)
}

func hsmGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		ba, baerr := json.Marshal(&Groups)
		if baerr != nil {
			log.Printf("ERROR: problem marshalling component list: '%v'\n", baerr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(ba)
	} else if r.Method == "POST" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Printf("ERROR: problem reading request body: '%v'\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = json.Unmarshal(body, &Groups)
		if err != nil {
			log.Printf("ERROR: problem unmarshalling request body: '%v'\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	} else {
		log.Printf("ERROR: request is not a GET or POST.\n")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
}

func doStateComponentsGet(w http.ResponseWriter, r *http.Request) {
	log.Printf(">> doStateComponentsGet")
	q := r.URL.Query()
	ctype, ctypeOK := q["type"]
	cstate, cstateOK := q["state"]
	_, csonlyOK := q["stateonly"]

	if !csonlyOK && !ctypeOK && !cstateOK {
		ba, baerr := json.Marshal(&Components)
		if baerr != nil {
			log.Printf("ERROR: problem marshalling component list: '%v'\n", baerr)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(ba)
		return
	}

	if !csonlyOK || !ctypeOK /* || !cstateOK */ {
		log.Printf("ERROR: request query must include 'type' and 'stateonly'.")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var jdata hsmStateArray
	var jd hsmStateOnly
	var ok bool

	for _, cmp := range Components.Components {
		ok = false
		for _, ct := range ctype {
			if cmp.Type == ct {
				log.Printf("Matched %s type: '%s'", cmp.ID, ct)
				ok = true
				break
			}
		}
		if !ok {
			continue
		}
		if cstateOK {
			ok = false
			for _, cs := range cstate {
				if cmp.State == cs {
					log.Printf("Matched %s state: '%s'", cmp.ID, cs)
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		jd.Flag = cmp.Flag
		jd.ID = cmp.ID
		jd.Type = cmp.Type
		jd.State = cmp.State
		jdata.Components = append(jdata.Components, jd)
	}

	ba, baerr := json.Marshal(&jdata)
	if baerr != nil {
		log.Printf("ERROR marshalling state-only components: %v", baerr)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(ba)
}

func subsRcv(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		log.Printf("ERROR: request is not a POST.\n")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var jdata ScnSubscribe
	body, err := ioutil.ReadAll(r.Body)
	err = json.Unmarshal(body, &jdata)
	if err != nil {
		log.Println("ERROR unmarshaling data:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("=================================================\n")
	log.Printf("Received an SCN subscription:\n")
	log.Printf("    Subscriber: %s\n", jdata.Subscriber)
	log.Printf("    Url:        %s\n", jdata.Url)
	if len(jdata.States) > 0 {
		log.Printf("    States:     '%s'\n", jdata.States[0])
		for ix := 1; ix < len(jdata.States); ix++ {
			log.Printf("                '%s'\n", jdata.States[ix])
		}
	}
	if len(jdata.SoftwareStatus) > 0 {
		log.Printf("    SWStatus:   '%s'\n", jdata.SoftwareStatus[0])
		for ix := 1; ix < len(jdata.SoftwareStatus); ix++ {
			log.Printf("                '%s'\n", jdata.SoftwareStatus[ix])
		}
	}
	if len(jdata.Roles) > 0 {
		log.Printf("    Roles:      '%s'\n", jdata.Roles[0])
		for ix := 1; ix < len(jdata.Roles); ix++ {
			log.Printf("                '%s'\n", jdata.Roles[ix])
		}
	}
	if jdata.Enabled != nil {
		log.Printf("    Enabled:    %t\n", *jdata.Enabled)
	}
	log.Printf("\n")
	log.Printf("=================================================\n")
	w.WriteHeader(http.StatusOK)
}

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type Routes []Route

func newRouter(routes []Route) *mux.Router {
	router := mux.NewRouter().StrictSlash(true)
	for _, route := range routes {
		var handler http.Handler
		handler = route.HandlerFunc
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(handler)
	}
	return router
}

// Create the API route descriptors.

func generateRoutes() Routes {
	return Routes{
		Route{"subsRcv",
			strings.ToUpper("POST"),
			"/hsm/v2/Subscriptions/SCN",
			subsRcv,
		},
		//Route{"hsmComponentsGet",
		//	strings.ToUpper("Get"),
		//	"/hsm/v2/State/Components",
		//	hsmComponentsGet,
		//},
		Route{"hsmComponentsPost",
			strings.ToUpper("Post"),
			"/hsm/v2/State/Components",
			hsmComponentsPost,
		},
		Route{"hsmComponentsXGet",
			strings.ToUpper("Get"),
			"/hsm/v2/State/Components/{xname}",
			hsmComponentsXGet,
		},
		Route{"hsmComponentsXPatch",
			strings.ToUpper("Patch"),
			"/hsm/v2/State/Components/{xname}/StateData",
			hsmComponentsXPatch,
		},
		Route{"hsmComponentsXPut",
			strings.ToUpper("Put"),
			"/hsm/v2/State/Components/{xname}",
			hsmComponentsXPut,
		},
		Route{"hsmGroups",
			strings.ToUpper("Get"),
			"/hsm/v2/groups",
			hsmGroups,
		},
		Route{"doReady",
			strings.ToUpper("Get"),
			"/hsm/v2/service/ready",
			doReady,
		},
		Route{"doStateComponentsGet",
			strings.ToUpper("Get"),
			"/hsm/v2/State/Components",
			doStateComponentsGet,
		},
		Route{"doBulkStateUpdate",
			strings.ToUpper("Patch"),
			"/hsm/v2/State/Components/BulkStateData",
			doBulkStateUpdate,
		},
		Route{"doServiceReady",
			strings.ToUpper("Get"),
			"/hsm/v2/service/ready",
			doServiceReady,
		},
	}
}

func main() {
	var envstr string
	port := ":27999"

	envstr = os.Getenv("PORT")
	if envstr != "" {
		port = envstr
	}

	routes := generateRoutes()
	router := newRouter(routes)

	log.Printf("==> Listening on port '%s'", port)
	for _, ep := range routes {
		log.Printf("    ===> %5s %s", ep.Method, ep.Pattern)
	}

	srv := &http.Server{Addr: port, Handler: router}

	err := srv.ListenAndServe()
	if err != nil {
		log.Println("ERROR firing up HTTP:", err)
		os.Exit(1)
	}

	os.Exit(0)
}
