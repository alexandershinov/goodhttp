package main

import (
	"time"
	"fmt"
	"github.com/alexandershinov/goodhttp"
	"bytes"
)

func main() {
	c := goodhttp.NewClient()
	var timeout time.Duration = 3 * time.Second
	c.SetConnectionTimeout(timeout)
	metric := []byte(`{"example": "OK"}`)
	response, err := c.GoodPost("https://github.com", "application/json", bytes.NewBuffer(metric))
	if err != nil {
		panic(err)
	}
	if response.StatusCode >= 400 {
		panic(response.Status)
	}
	fmt.Println(response.StatusCode)
}
