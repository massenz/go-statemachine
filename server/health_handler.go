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
    "net/http"
)

// NOTE: We make the handlers "exportable" so they can be tested, do NOT call directly.

type HealthResponse struct {
    Status  string `json:"status"`
    Release string `json:"release"`
}

func HealthHandler(w http.ResponseWriter, r *http.Request) {
    // Standard preamble for all handlers, sets tracing (if enabled) and default content type.
    defer trace(r.RequestURI)()
    defaultContent(w)

    var response MessageResponse
    res := HealthResponse{
        Status:  "OK",
        Release: Release,
    }
    if err := storeManager.Health(); err != nil {
        logger.Error("Health check failed: %s", err)
        res.Status = "ERROR"
        response = MessageResponse{
            Msg:   res,
            Error: fmt.Sprintf("error connecting to storage: %s", err),
        }
    } else {
        response = MessageResponse{
            Msg: res,
        }
    }
    err := json.NewEncoder(w).Encode(response)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
}
