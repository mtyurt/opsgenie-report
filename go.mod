module opsgenie-report

go 1.13

require (
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/franela/goblin v0.0.0-20200611003024-99f9a98191cf // indirect
	github.com/franela/goreq v0.0.0-20171204163338-bcd34c9993f8 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/onsi/gomega v1.10.1 // indirect
	github.com/opsgenie/opsgenie-go-sdk v0.0.0-20181102130742-d57b8391ca90
)

replace github.com/opsgenie/opsgenie-go-sdk => github.com/mtyurt/opsgenie-go-sdk v0.0.0-20191101090055-9b54081d5e56
