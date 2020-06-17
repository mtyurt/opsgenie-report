package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/opsgenie/opsgenie-go-sdk/alertsv2"

	ogcli "github.com/opsgenie/opsgenie-go-sdk/client"
)

func humanReadable(ms int) string {
	seconds := (ms / 1000) % 60
	minutes := (ms / (1000 * 60)) % 60
	hours := (ms / (1000 * 60 * 60)) % 24
	days := ms / (1000 * 60 * 60 * 24)
	return fmt.Sprintf("%d days, %d hours, %d minutes, %d seconds", days, hours, minutes, seconds)
}

type responderAckTime struct {
	totalAck int
	count    int
}

type searchQuery struct {
	startDate   time.Time
	endDate     time.Time
	alertStatus string
}

func main() {
	startFrom := flag.Int("startdays", 0, "Amount of days before today to start the report from")
	days := flag.Int("days", 7, "Amount of days the report should cover")
	status := flag.String("status", "all", "Status of alerts to cover")
	afterHours := flag.Bool("afterhours", false, "Separate metrics for after business hours")
	location := flag.String("location", "UTC", "The location to check after hours")
	blame := flag.Bool("blame", false, "Show each responder's metrics as well")
	flag.Parse()

	query := searchQuery{
		time.Now().Add(time.Duration((*days+*startFrom)*-24) * time.Hour),
		time.Now().Add(time.Duration(*startFrom*-24) * time.Hour),
		*status,
	}

	fmt.Printf("Preparing opsgenie report from %v to %v...\n", query.startDate.Format(time.UnixDate), query.endDate.Format(time.UnixDate))

	alerts, err := getAlerts(query)
	if err != nil {
		panic(err)
	}
	if *afterHours {
		fmt.Println("Separating alerts for after business hours...")
		timeLoc, err := time.LoadLocation(*location)
		if err != nil {
			panic(err)
		}

		businessHoursAlerts := make([]alertsv2.Alert, 0)
		afterHoursAlerts := make([]alertsv2.Alert, 0)
		for _, alert := range alerts {
			hour := alert.CreatedAt.In(timeLoc).Hour()
			if hour >= 9 && hour < 18 {
				businessHoursAlerts = append(businessHoursAlerts, alert)
			} else {
				afterHoursAlerts = append(afterHoursAlerts, alert)
			}
		}

		fmt.Printf("\n## Business hour alerts (9 AM to 6 PM in %s)\n", *location)
		prepareReport(businessHoursAlerts, *blame)
		fmt.Printf("\n## After hour alerts (6 PM to 9 AM in %s)\n", *location)
		prepareReport(afterHoursAlerts, *blame)
	} else {
		prepareReport(alerts, *blame)
	}

}

func prepareReport(alerts []alertsv2.Alert, blame bool) {
	totalAck := 0
	totalClose := 0
	ackTimeByResponder := make(map[string]*responderAckTime)

	for _, alert := range alerts {
		totalAck += int(alert.Report.AckTime)
		totalClose += int(alert.Report.CloseTime)

		if blame {

			acknowledger := strings.Split(alert.Report.AcknowledgedBy, "@")[0]
			if acknowledger == "" {
				continue
			}
			if responder, ok := ackTimeByResponder[acknowledger]; ok {
				responder.count++
				responder.totalAck += int(alert.Report.AckTime)
			} else {
				ackTimeByResponder[acknowledger] = &responderAckTime{int(alert.Report.AckTime), 1}
			}
		}
	}

	if len(alerts) == 0 {
		fmt.Println("No alerts found")
		return
	}
	fmt.Printf("MTTA for %d alerts:\n", len(alerts))
	fmt.Println(humanReadable(totalAck / len(alerts)))

	fmt.Printf("MTTR for %d alerts:\n", len(alerts))
	fmt.Println(humanReadable(totalClose / len(alerts)))

	if blame {
		fmt.Println("\nMTTA per responder:")
		for name, responder := range ackTimeByResponder {
			fmt.Printf(" - %s: %s for %d alerts\n", name, humanReadable(responder.totalAck/responder.count), responder.count)
		}
	}

}

func getAlerts(query searchQuery) ([]alertsv2.Alert, error) {
	cli := new(ogcli.OpsGenieClient)
	apiKey := os.Getenv("GENIEKEY")
	cli.SetAPIKey(apiKey)
	cli.SetOpsGenieAPIUrl("https://api.eu.opsgenie.com")

	alertCli, err := cli.AlertV2()
	if err != nil {
		return nil, fmt.Errorf("Creating alert cli failed: %w", err)
	}

	queryStr := fmt.Sprintf("createdAt>%d and createdAt<%d", epochMs(query.startDate), epochMs(query.endDate))
	if query.alertStatus != "" && query.alertStatus != "all" {
		queryStr = "status: " + query.alertStatus + " and " + queryStr
	}
	countResp, err := alertCli.Count(alertsv2.CountAlertRequest{Query: queryStr})
	fmt.Printf("Total found alert count: %d\n", countResp.AlertCount.Count)
	if err != nil {
		return nil, fmt.Errorf("Fetching total count has failed %w", err)
	}

	fmt.Printf("Total count of found alerts: %d\n", countResp.AlertCount.Count)

	alerts := make([]alertsv2.Alert, 0, countResp.AlertCount.Count)
	for totalCount := 0; totalCount < countResp.AlertCount.Count; {
		req := alertsv2.ListAlertRequest{
			Query:  queryStr,
			Sort:   alertsv2.CreatedAt,
			Order:  alertsv2.Asc,
			Offset: totalCount,
			Limit:  100,
		}
		resp, err := alertCli.List(req)
		if err != nil {
			return nil, fmt.Errorf("Fetching alerts has failed for offset. %w", err)
		}
		alerts = append(alerts, resp.Alerts...)
		totalCount += 100
		if totalCount < countResp.AlertCount.Count {
			time.Sleep(time.Second * 1)
		}
	}

	return alerts, nil
}

func epochMs(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}
