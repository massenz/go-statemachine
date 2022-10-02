/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Author: Marco Massenzio (marco@alertavert.com)
 */

package server

import (
    "encoding/json"
    "fmt"
    "github.com/gorilla/mux"
    "net/http"
)

func GetEventHandler(w http.ResponseWriter, r *http.Request) {
    defer trace(r.RequestURI)()
    defaultContent(w)

    // We don't really need to check for the presence of the parameter,
    // as the Mux router takes care of all the error handling for us.
    vars := mux.Vars(r)
    cfgName := vars["cfg_name"]
    evtId := vars["evt_id"]
    logger.Debug("Looking up Event: %s#%s", cfgName, evtId)

    event, ok := storeManager.GetEvent(evtId, cfgName)
    if !ok {
        http.Error(w, fmt.Sprintf("Event [%s] not found", evtId), http.StatusNotFound)
        return
    }
    logger.Debug("Found Event: %s", event.String())

    err := json.NewEncoder(w).Encode(&EventResponse{ID: evtId, Event: event})
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}

func GetOutcomeHandler(w http.ResponseWriter, r *http.Request) {
    defer trace(r.RequestURI)()
    defaultContent(w)

    vars := mux.Vars(r)
    cfgName := vars["cfg_name"]
    evtId := vars["evt_id"]
    logger.Debug("Looking up Outcome for Event: %s#%s", cfgName, evtId)

    outcome, ok := storeManager.GetOutcomeForEvent(evtId, cfgName)
    if !ok {
        http.Error(w, fmt.Sprintf("Outcome for Event [%s] not found", evtId), http.StatusNotFound)
        return
    }
    logger.Debug("Found Event Outcome: %s", outcome.String())
    err := json.NewEncoder(w).Encode(MakeOutcomeResponse(outcome))
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
    }
}
