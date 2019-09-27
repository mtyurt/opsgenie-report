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
	days := flag.Int("days", 7, "Amount of days the report should cover")
	status := flag.String("status", "closed", "Amount of days the report should cover")
	afterHours := flag.Bool("afterhours", false, "Separate metrics for after business hours")
	location := flag.String("location", "UTC", "The location to check after hours")
	flag.Parse()

	fmt.Printf("Preparing opsgenie report for %d days...\n", *days)

	query := searchQuery{
		time.Now().Add(time.Duration(*days*-24) * time.Hour),
		time.Now(),
		*status,
	}

	response, err := getAlerts(query)
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
		for _, alert := range response.Alerts {
			hour := alert.CreatedAt.In(timeLoc).Hour()
			if hour >= 9 && hour < 18 {
				businessHoursAlerts = append(businessHoursAlerts, alert)
			} else {
				afterHoursAlerts = append(afterHoursAlerts, alert)
			}
		}

		fmt.Printf("\n## Business hour alerts (9 AM to 6 PM in %s)\n", *location)
		prepareReport(businessHoursAlerts)
		fmt.Printf("\n## After hour alerts (6 PM to 9 AM in %s)\n", *location)
		prepareReport(afterHoursAlerts)
	} else {
		prepareReport(response.Alerts)
	}

}

func prepareReport(alerts []alertsv2.Alert) {
	totalAck := 0
	totalClose := 0
	ackTimeByResponder := make(map[string]*responderAckTime)

	for _, alert := range alerts {
		totalAck += int(alert.Report.AckTime)
		totalClose += int(alert.Report.CloseTime)

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

	if len(alerts) == 0 {
		fmt.Println("No alerts found")
		return
	}
	fmt.Printf("MTTA for %d alerts:\n", len(alerts))
	fmt.Println(humanReadable(totalAck / len(alerts)))

	fmt.Printf("MTTR for %d alerts:\n", len(alerts))
	fmt.Println(humanReadable(totalClose / len(alerts)))

	fmt.Println("\nMTTA per responder:")
	for name, responder := range ackTimeByResponder {
		fmt.Printf(" - %s: %s for %d alerts\n", name, humanReadable(responder.totalAck/responder.count), responder.count)
	}

}

func getAlerts(query searchQuery) (*alertsv2.ListAlertResponse, error) {
	cli := new(ogcli.OpsGenieClient)
	apiKey := os.Getenv("GENIEKEY")
	cli.SetAPIKey(apiKey)
	cli.SetOpsGenieAPIUrl("https://api.eu.opsgenie.com")

	req := alertsv2.ListAlertRequest{
		Query: fmt.Sprintf("createdAt>%d createdAt<%d", epochMs(query.startDate), epochMs(query.endDate)),
		Sort:  alertsv2.CreatedAt,
		Order: alertsv2.Asc,
	}
	if query.alertStatus != "" && query.alertStatus != "all" {
		req.Query = "status: " + query.alertStatus + " " + req.Query
	}

	alertCli, err := cli.AlertV2()
	if err != nil {
		return nil, fmt.Errorf("Creating alert cli failed: %w", err)
	}

	return alertCli.List(req)
}

func epochMs(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}
