# go-workingday
### 判断某天是不是工作日，目前支持🇨🇳大陆, 🇨🇳台湾， 🇨🇳澳门，🇨🇳香港
 *由于国务院放假安排每年可能都不一样，所以仅支持年以及三年前的数据，后续版本可能将支持手动添加放假数据*
### a golang package to get workday info, currently suport cn, tw, ma, hk region
*as holiday arrangement is defferent every year, it's arranged accroading to related govement files, we only support current year and passed 3 years calculation, may add manual data in the later versions*

### 使用示例（example）：
```go
package main

import (
	"github.com/Admingyu/go-workingday"
	"log"
	"time"
)

func main() {
	dt := time.Now()
	region := "CN"
	isWork, dayType := workingday.IsWorkDay(dt, region)

	log.Print("现在是：", dt)
	log.Print("地区：", region)
	log.Print("今天需要上班？", isWork)
	log.Print("原因：", dayType)
}
```
*数据来源：http://www.suishenyun.net*
