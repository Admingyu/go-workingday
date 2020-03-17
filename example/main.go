package main

import (
	"github.com/Admingyu/go-workingday"
	"log"
	"time"
)

func main() {
	isWork, dayType := workingday.IsWorkDay(time.Now(), "CN")
	log.Print("是否上班：", isWork, "， 原因：", dayType)
}
