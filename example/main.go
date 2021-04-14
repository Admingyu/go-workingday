package main

import (
	"log"
	"time"

	"github.com/Admingyu/go-workingday"
)

func main() {
	isWork, dayType := workingday.IsWorkDay(time.Now(), "CN")
	date := workingday.NthWorkdayFromLast(time.Now(), 3, "MA")
	log.Println(date)
	log.Print("是否上班：", isWork, "， 原因：", dayType)
}
