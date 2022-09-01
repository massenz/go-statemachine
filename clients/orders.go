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

package main

import (
    "encoding/json"
    "time"
)

type OrderDetails struct {
    OrderId    string
    CustomerId string
    OrderDate  time.Time
    OrderTotal float64
}

func NewOrderDetails(orderId, customerId string, orderTotal float64) *OrderDetails {
    return &OrderDetails{
        OrderId:    orderId,
        CustomerId: customerId,
        OrderDate:  time.Now(),
        OrderTotal: orderTotal,
    }
}

func (o *OrderDetails) String() string {
    res, error := json.Marshal(o)
    if error != nil {
        panic(error)
    }
    return string(res)
}
