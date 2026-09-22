# go-workingday

用于判断某一天是否为工作日，并计算月末倒数第 N 个工作日的 Go 库。

项目采用离线优先策略：默认读取随包发布的数据文件，不依赖网络；调用方也可以导入自己的离线文件，或显式加载线上最新数据。

## 版本选择

| 版本 | 导入路径 | 地区 | 离线格式 | 适用场景 |
| --- | --- | --- | --- | --- |
| v2（推荐） | `github.com/Admingyu/go-workingday/v2` | 中国大陆 `CN` | ICS | 新项目；提供严格的数据校验、覆盖年份检查和错误返回 |
| v1（兼容） | `github.com/Admingyu/go-workingday` | `CN`、`HK`、`MA`、`TW` | JSON | 兼容原有 API 或接口 JSON；各地区历史数据范围并不一致 |

最低 Go 版本为 1.16。

## 安装

```bash
go get github.com/Admingyu/go-workingday
```

## v2 快速开始

```go
package main

import (
	"log"
	"time"

	workingday "github.com/Admingyu/go-workingday/v2"
)

func main() {
	// LoadCalendar 只读取随包发布的 ICS，不会访问网络。
	calendar, err := workingday.LoadCalendar(workingday.RegionCN)
	if err != nil {
		log.Fatal(err)
	}

	isWorkday, status, err := calendar.IsWorkDay(time.Now())
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("是否上班：%v，状态：%s", isWorkday, status)
}
```

状态含义：

| 状态 | 含义 | 是否工作日 |
| --- | --- | --- |
| `NORMAL` | 没有调休安排，按照星期判断 | 周一至周五是，周六和周日否 |
| `WORK` | 周末调为补班日 | 是 |
| `REST` | 调为假期 | 否 |

### 导入自己的 ICS 文件

```go
calendar, err := workingday.LoadCalendarFromFile(
	"/path/to/holidayCal.ics",
	workingday.RegionCN,
)
if err != nil {
	log.Fatal(err)
}
```

v2 会严格校验 ICS：

- `X-WR-CALDESC` 必须包含 `YYYY~YYYY年` 格式的完整覆盖范围；
- 每个覆盖年份必须至少有一个事件；
- `SUMMARY` 必须能识别为“假期/放假”或“补班”；
- 同一天不能同时声明为假期和补班。

不符合这些约束时会返回错误，不会静默使用网络数据兜底。

### 显式加载线上数据

```go
ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
defer cancel()

calendar, err := workingday.LoadCalendarOnline(ctx, workingday.RegionCN)
if err != nil {
	log.Fatal(err)
}
```

`LoadCalendarOnline` 只将线上数据加载到当前进程内存，不会改写仓库或二进制中的离线快照。

### 计算月末工作日

```go
thirdFromLast, err := calendar.NthWorkdayFromLast(time.Now(), 3)
if err != nil {
	log.Fatal(err)
}

// 等价的便捷方法：
thirdFromLast, err = calendar.LastThirdWorkday(time.Now())
```

如果 `n <= 0`、年份不在数据覆盖范围内，或当月没有足够的工作日，方法会返回错误，不会跨到上一个月继续计算。

可通过 `calendar.Years()` 查看当前 ICS 声明的完整覆盖年份。

## v1 使用方式

新代码即使继续使用 v1，也建议使用能够返回错误的 `Calendar` API：

```go
package main

import (
	"log"
	"time"

	workingday "github.com/Admingyu/go-workingday"
)

func main() {
	// LoadCalendar 读取内置的 data/festival.json，不会访问网络。
	calendar, err := workingday.LoadCalendar()
	if err != nil {
		log.Fatal(err)
	}

	isWorkday, status, err := calendar.IsWorkDay(time.Now(), "CN")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("是否上班：%v，状态：%s", isWorkday, status)
}
```

原有函数继续可用，并改为读取内置离线数据：

```go
isWorkday, status := workingday.IsWorkDay(time.Now(), "CN")
thirdFromLast := workingday.NthWorkdayFromLast(time.Now(), 3, "CN")
```

这些旧函数没有 `error` 返回值，数据或参数异常时会 panic。需要自行处理错误的代码应使用 `Calendar` API。

### 导入自己的 JSON 文件

```go
calendar, err := workingday.LoadCalendarFromFile("/path/to/festival.json")
if err != nil {
	log.Fatal(err)
}
```

JSON 文件沿用旧接口的相关字段格式，并要求四个地区数组都存在且非空：

```json
{
  "national_holiday": {},
  "holidays": {
    "cn": [{"date": 20260101, "status": 0}],
    "hk": [{"date": 20260101, "status": 0}],
    "ma": [{"date": 20260101, "status": 0}],
    "tw": [{"date": 20260101, "status": 0}]
  }
}
```

其中 `status: 0` 表示休息，`status: 1` 表示补班。日期必须是有效的 `YYYYMMDD` 整数。

显式加载旧接口数据：

```go
calendar, err := workingday.LoadCalendarOnline(ctx)
```

同样地，线上加载不会改写内置 JSON 快照。

## 离线数据与覆盖范围

内置数据于 2026-09-22 获取：

| 版本 | 文件 | 数据来源 |
| --- | --- | --- |
| v1 | [`data/festival.json`](data/festival.json) | [随身云 festival 接口](http://pc.suishenyun.net/peacock/api/h5/festival) |
| v2 | [`v2/data/holidayCal.ics`](v2/data/holidayCal.ics) | [china-holiday-calender](https://github.com/lanceliao/china-holiday-calender) |

v1 快照中各地区实际保存的日期条目范围如下。这里只表示文件中最早和最晚的调休记录，不代表中间每一天都有事件，也不代表数据源提供完整的全年覆盖声明。

| 地区 | 最早条目 | 最晚条目 |
| --- | --- | --- |
| `CN` | 2022-01-01 | 2026-10-10 |
| `HK` | 2016-12-31 | 2017-12-26 |
| `MA` | 2016-01-01 | 2016-12-25 |
| `TW` | 2017-01-01 | 2017-10-10 |

v2 内置 ICS 明确声明的完整覆盖年份为 2023–2026。查询未覆盖年份时，v2 会返回错误。

数据文件的维护说明见 [`data/README.md`](data/README.md)。

## 加载规则

库不会在不同来源之间进行隐式降级：

1. `LoadCalendar` 只读取内置离线数据；
2. `LoadCalendarFromFile` 只读取调用方指定的文件；
3. `LoadCalendarOnline` 只读取线上数据。

因此，离线文件格式错误、文件不存在或线上请求失败时，调用方会收到对应错误，而不是得到来源不明或可能过期的结果。

## 开发验证

```bash
go test -mod=readonly ./...
go test -race -mod=readonly ./...
go vet -mod=readonly ./...
```

## License

[MIT](LICENSE)
