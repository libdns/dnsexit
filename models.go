package dnsexit

import (
	"fmt"
	"time"

	"github.com/libdns/libdns"
)

type Action int64

const (
	setRecords Action = iota
	appendRecords
	deleteRecords
)

type dnsExitPayload struct {
	Apikey        string           `json:"apikey"`
	Zone          string           `json:"domain"`
	AddRecords    *[]dnsExitRecord `json:"add,omitempty"`
	DeleteRecords *[]dnsExitRecord `json:"delete,omitempty"`
}
type dnsExitRecord struct {
	Type       string  `json:"type"`
	Name       string  `json:"name,omitempty"`
	Content    *string `json:"content,omitempty"`
	Priority   *uint16 `json:"priority,omitempty"`
	TTL        *int    `json:"ttl,omitempty"`
	Overwrite  *bool   `json:"overwrite,omitempty"`
	MailZone   string  `json:"mail-zone,omitempty"`   // "mail-zone":"",
	MailServer string  `json:"mail-server,omitempty"` // "mail-server":"mail2.dnsexit.com",

}

// Correct struct tags for the actual API response
type dnsExitResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

func createDnsExitRecord(rr libdns.RR, zone string, action Action) (dnsExitRecord, error) {

	//Convert TTL from time.Duration to minutes
	ttlInMinutes := int(rr.TTL / time.Second)

	relativeName := libdns.RelativeName(rr.Name, zone)
	trimmedName := relativeName
	if relativeName == "@" {
		trimmedName = ""
	}

	var currentRecord dnsExitRecord
	currentRecord.Type = rr.Type
	currentRecord.Name = trimmedName
	var mx libdns.MX
	if rr.Type == "MX" {
		parsed, err := rr.Parse()
		if err != nil {
			return dnsExitRecord{}, fmt.Errorf("failed to convert record to MX type: %v", rr)
		}
		if debug {
			fmt.Println("MX record found:")
			fmt.Println(parsed)
			fmt.Println("Name: " + parsed.(libdns.MX).Name)
			fmt.Println("Preference: " + fmt.Sprintf("%d", parsed.(libdns.MX).Preference))
			fmt.Println("Target: " + parsed.(libdns.MX).Target)
		}
		mx = parsed.(libdns.MX)
		// For add/set MX records, set Name to target. For delete, keep the record name and use MailServer for target.
		if action != deleteRecords {
			currentRecord.Name = mx.Target
		}
	}

	if action != deleteRecords {
		if rr.Type == "MX" {
			currentRecord.Priority = &mx.Preference
			currentRecord.MailServer = mx.Target
			//TODO - Not clear how MailZone is specified in libdns
			// currentRecord.MailZone = &mx.Name
		} else {
			recordValue := rr.Data
			currentRecord.Content = &recordValue
		}
		currentRecord.TTL = &ttlInMinutes
	} else {
		// For delete operations, MX records need Priority and MailServer, others need Content
		if rr.Type == "MX" {
			currentRecord.Priority = &mx.Preference
			currentRecord.MailServer = mx.Target
		} else {
			recordValue := rr.Data
			currentRecord.Content = &recordValue
		}
	}
	if action == setRecords {
		truevalue := true
		currentRecord.Overwrite = &truevalue
	}

	return currentRecord, nil
}
