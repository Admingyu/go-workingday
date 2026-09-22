# Offline data

The library ships with package-local snapshots so normal queries do not depend on network availability.

- `festival.json` is used by the v1 package. It contains the `national_holiday` and `holidays` fields from `http://pc.suishenyun.net/peacock/api/h5/festival`.
- `../v2/data/holidayCal.ics` is used by the v2 package. It comes from `https://cdn.jsdelivr.net/gh/lanceliao/china-holiday-calender/holidayCal.ics`.

The checked-in snapshots were refreshed on 2026-09-22. Updating a snapshot is an explicit maintenance operation; runtime queries always prefer the embedded files. Applications can import their own snapshot with `LoadCalendarFromFile`, or explicitly request the online source with `LoadCalendarOnline`.
