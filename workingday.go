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
	Cn []dayType `json:"cn"`
	Hk []dayType `json:"hk"`
	Ma []dayType `json:"ma"`
	Tw []dayType `json:"tw"`
}

type calendarBody struct {
	NationalHoliday interface{}   `json:"national_holiday"`
	Holidays        *holidaysType `json:"holidays"`
}

// 获取假日表并解析
func FillCalendar() calendarBody {
	resp, err := http.Get("http://pc.suishenyun.net/peacock/api/h5/festival")
	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()
	body_s, err := ioutil.ReadAll(resp.Body)

	cb := calendarBody{}
	err = json.Unmarshal(body_s, &cb)
	if err != nil {
		log.Fatalln(err)
	}
	return cb
}

// 判断今天是不是工作日:
// dataIn 当前时间
// region 地区（CN：中国大陆。 HK：中国香港， MA：中国澳门， TW:中国台湾）
// 返回参数：是否是工作日（true：上班， false：不上班），当前状态：（NORMAL：正常，WORK：调休上班，REST：假期）
func IsWorkDay(dateIn time.Time, region string) (bool, string) {

	// 计算到期日期上个月的日期
	var needWork bool
	var holidayData []dayType
	lastdayStr := dateIn.Format("20060102")
	holidayData = GetRegionHolidays(region)

	// 调休状态：“NORMAL”：未调休，“REST”：调成休息， “WORK”：调成上班
	shiftStatus := "NORMAL"
	for _, v := range holidayData {
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
func LastThirdWorkDay(datetime time.Time) time.Time {
	return NthWorkdayFromLast(datetime, 3, "CN")
}

//获取指定日期所属月份的倒数第n个工作日
func NthWorkdayFromLast(datetime time.Time, n int, region string) time.Time {

	firstday := time.Date(datetime.Year(), datetime.Month(), 1, 0, 0, 0, 0, time.Local)
	lastday := firstday.AddDate(0, 1, 0).Add(time.Second * -1)
	holidayData := GetRegionHolidays(region)

	var workdays []time.Time
	for len(workdays) < n {
		lastdayStr := lastday.Format("20060102")
		// 调休状态：“NORMAL”：未调休，“REST”：调成休息， “WORK”：调成上班
		shiftStatus := "NORMAL"
		for _, v := range holidayData {
			if lastdayStr == fmt.Sprintf("%d", v.Date) {
				if v.Status == 0 {
					shiftStatus = "REST"
				} else if v.Status == 1 {
					shiftStatus = "WORK"
					workdays = append(workdays, lastday)
				}
				break
			}
		}

		// 未调休,并且不是周六或者周日
		if shiftStatus == "NORMAL" && lastday.Weekday() != time.Sunday && lastday.Weekday() != time.Saturday {
			shiftStatus = "WORK"
			workdays = append(workdays, lastday)
		} else if shiftStatus == "NORMAL" {
			shiftStatus = "REST"
		}
		lastday = lastday.AddDate(0, 0, -1)
	}

	return workdays[n-1]
}

// 获取某个地区假日数据
// region: 地区（CN：中国大陆。 HK：中国香港， MA：中国澳门， TW:中国台湾）
func GetRegionHolidays(region string) []dayType {
	calendar := FillCalendar()
	regionCalMap := map[string][]dayType{
		"CN": calendar.Holidays.Cn,
		"HK": calendar.Holidays.Hk,
		"MA": calendar.Holidays.Ma,
		"TW": calendar.Holidays.Tw,
	}
	return regionCalMap[region]
}
