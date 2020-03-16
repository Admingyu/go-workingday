package main

import (
	"github.com/Admingyu/go-workingday"
	"log"
	"time"
)

func main() {
	log.Println(workingday.IsWorkDay(time.Now()))
}
