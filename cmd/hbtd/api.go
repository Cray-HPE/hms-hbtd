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
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

type Route struct {
	Name        string
	Method      string
	Pattern     string
	HandlerFunc http.HandlerFunc
}

type Routes []Route

const (
	URL_BASE      = "/hmi"
	URL_VERSION   = "/v1"
	URL_ROOT      = URL_BASE + URL_VERSION
	URL_HEARTBEAT = URL_ROOT + "/heartbeat"
	URL_PARAMS    = URL_ROOT + "/params"
	URL_HB_STATES = URL_ROOT + "/hbstates"
	URL_HB_STATE  = URL_ROOT + "/hbstate"
	URL_LIVENESS  = URL_ROOT + "/liveness"
	URL_READINESS = URL_ROOT + "/readiness"
	URL_HEALTH    = URL_ROOT + "/health"
)

// Generate the API routes
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
		Route{"hbRcv",
			strings.ToUpper("Post"),
			URL_HEARTBEAT,
			hbRcv,
		},
		Route{"hbRcvXName",
			strings.ToUpper("Post"),
			URL_HEARTBEAT + "/{xname}",
			hbRcvXName,
		},
		Route{"params_get",
			strings.ToUpper("Get"),
			URL_PARAMS,
			paramsIO,
		},
		Route{"params_patch",
			strings.ToUpper("Patch"),
			URL_PARAMS,
			paramsIO,
		},
		Route{"doHealth",
			strings.ToUpper("Get"),
			URL_HEALTH,
			doHealth,
		},
		Route{"doLiveness",
			strings.ToUpper("Get"),
			URL_LIVENESS,
			doLiveness,
		},
		Route{"doReadiness",
			strings.ToUpper("Get"),
			URL_READINESS,
			doReadiness,
		},
		Route{"hbStates",
			strings.ToUpper("Post"),
			URL_HB_STATES,
			hbStates,
		},
		Route{"hbStateSingle",
			strings.ToUpper("Get"),
			URL_HB_STATE + "/{xname}",
			hbStateSingle,
		},
	}
}
