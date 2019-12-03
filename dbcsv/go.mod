module github.com/tgulacsi/go/dbcsv

require (
	github.com/360EntSecGroup-Skylar/excelize/v2 v2.0.1
	github.com/extrame/goyymmdd v0.0.0-20181026012948-914eb450555b // indirect
	github.com/extrame/xls v0.0.2-0.20180905092746-539786826ced
	github.com/tgulacsi/go v0.0.0-00010101000000-000000000000
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/text v0.3.2
	golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7
	gopkg.in/goracle.v2 v2.23.4
)

go 1.13

//replace gopkg.in/goracle.v2 => ../../../../gopkg.in/goracle.v2
replace github.com/tgulacsi/go => ../
