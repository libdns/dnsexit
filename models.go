package dnsexit

import (
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

// TODO - look at co-ercing properties of LibDns mx records into MailZone/MailServer properties
// MailZone   string `json:"mail-zone,omitempty"`   // "mail-zone":"",
// MailServer string `json:"mail-server,omitempty"` // "mail-server":"mail2.dnsexit.com",

type dnsExitRecord struct {
	Type      string  `json:"type"`
	Name      string  `json:"name,omitempty"`
	Content   *string `json:"content,omitempty"`
	Priority  *int    `json:"priority,omitempty"`
	TTL       *int    `json:"ttl,omitempty"`
	Overwrite *bool   `json:"overwrite,omitempty"`
}

// Correct struct tags for the actual API response
type dnsExitResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

func createDnsExitRecord(rr libdns.RR, zone string, action Action) (dnsExitRecord, error) {

	if rr.TTL/time.Second < 600 {
		rr.TTL = 600 * time.Second
	}
	ttlInSeconds := int(rr.TTL / time.Second)

	relativeName := libdns.RelativeName(rr.Name, zone)
	trimmedName := relativeName
	if relativeName == "@" {
		trimmedName = ""
	}

	var currentRecord dnsExitRecord
	currentRecord.Type = rr.Type
	currentRecord.Name = trimmedName

	if action != deleteRecords {
		recordValue := rr.Data
		currentRecord.Content = &recordValue
		//TODO - determine how to parse priority, or if this is still needed
		// recordPriority := int(rr.Data.Priority)
		// currentRecord.Priority = &recordPriority
		recordTTL := ttlInSeconds
		currentRecord.TTL = &recordTTL
	}
	if action == setRecords {
		truevalue := true
		currentRecord.Overwrite = &truevalue
	}

	return currentRecord, nil
}
