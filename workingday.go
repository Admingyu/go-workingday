package workingday

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type dayType struct {
	Date   int `json:"date"`
	Status int `json:"status"`
}

type holidaysType struct {
	Cn []dayType   `json:"cn"`
	Hk interface{} `json:"hk"`
	Ma interface{} `json:"ma"`
	Tw interface{} `json:"tw"`
}

type calandarBody struct {
	NationalHoliday interface{}   `json:"national_holiday"`
	Holidays        *holidaysType `json:"holidays"`
}

// 获取假日表并解析
func FillCalandar() calandarBody {
	resp, err := http.Get("http://pc.suishenyun.net/peacock/api/h5/festival")

	if err != nil {
		log.Println(err)
	}

	defer resp.Body.Close()
	body_s, err := ioutil.ReadAll(resp.Body)

	cb := calandarBody{}
	err = json.Unmarshal(body_s, &cb)
	if err != nil {
		log.Fatalln(err)
	}
	return cb
}

// 判断今天是不是工作日
func IsWorkDay(dateIn time.Time) (bool, string) {

	// 计算到期日期上个月的日期

	Calandar := FillCalandar()
	var needWork bool
	lastdayStr := dateIn.Format("20060102")
	// 调休状态：“NORMAL”：未调休，“REST”：调成休息， “WORK”：调成上班
	shiftStatus := "NORMAL"
	for _, v := range Calandar.Holidays.Cn {
		if lastdayStr == fmt.Sprintf("%d", v.Date) {
			if v.Status == 0 {
				shiftStatus = "REST"
				needWork = false
			} else if v.Status == 1 {
				shiftStatus = "WORK"
				needWork = true
			}
			break
		}
	}

	// 未调休,并且不是周六或者周日
	if shiftStatus == "NORMAL" && dateIn.Weekday() != time.Sunday && dateIn.Weekday() != time.Saturday {
		needWork = true
	} else if shiftStatus == "NORMAL" {
		needWork = false
	}

	return needWork, shiftStatus
}

//获取日期月份倒数第三个工作日
func LastThirdWorkDay(loanDay time.Time) time.Time {

	// 计算到期日期上个月的日期
	lastMonthDay := time.Now()
	firstday := time.Date(lastMonthDay.Year(), lastMonthDay.Month(), 1, 0, 0, 0, 0, time.Local)
	lastday := firstday.AddDate(0, 1, 0).Add(time.Second * -1)

	Calandar := FillCalandar()

	var workdays []time.Time
	for len(workdays) < 3 {
		lastdayStr := lastday.Format("20060102")
		// 调休状态：“NORMAL”：未调休，“REST”：调成休息， “WORK”：调成上班
		shiftStatus := "NORMAL"
		for _, v := range Calandar.Holidays.Cn {
			if lastdayStr == fmt.Sprintf("%d", v.Date) {
				if v.Status == 0 {
					shiftStatus = "REST"
				} else if v.Status == 1 {
					shiftStatus = "WORK"
					workdays = append(workdays, lastday)
				}
				// log.Println(lastdayStr, shiftStatus)
				break
			}
		}

		// 未调休,并且不是周六或者周日
		if shiftStatus == "NORMAL" && lastday.Weekday() != time.Sunday && lastday.Weekday() != time.Saturday {
			// log.Println(shiftStatus, lastdayStr)
			shiftStatus = "WORK"
			workdays = append(workdays, lastday)
		} else if shiftStatus == "NORMAL" {
			shiftStatus = "REST"
		}
		lastday = lastday.AddDate(0, 0, -1)
	}

	return workdays[2]
}

func main() {
	log.Println("hello")
}
