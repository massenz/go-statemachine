/*
 * Copyright (c) 2022 AlertAvert.com.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0
 * http://www.apache.org/licenses/LICENSE-2.0
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
