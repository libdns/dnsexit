package dnsexit

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/libdns/libdns"
	"github.com/pkg/errors"
)

var (
	// Set environment variable to "TRUE" to enable debug logging
	debug = (os.Getenv("LIBDNS_DNSEXIT_DEBUG") == "TRUE")
)

func (p *Provider) effectiveZone(zone string) string {
	if p.Zone != "" {
		return p.Zone
	}
	return zone
}

// Query Google DNS for A/AAAA/TXT record for a given DNS name
func (p *Provider) getDomain(ctx context.Context, zone string) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	var libRecords []libdns.Record

	// The DNSExit API only supports adding/updating/deleting records and no way
	// to get current records. So instead, we just make
	// simple DNS queries to get the A, AAAA, and TXT records.
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 10 * time.Second}
			return d.DialContext(ctx, network, "8.8.8.8:53")
		},
	}

	ips, err := r.LookupHost(ctx, zone)
	if err != nil {
		var dnsErr *net.DNSError
		// Ignore missing dns record
		if !(errors.As(err, &dnsErr) && dnsErr.IsNotFound) {
			return libRecords, errors.Wrapf(err, "error looking up host")
		}
	}

	for _, ip := range ips {
		parsed, err := netip.ParseAddr(ip)
		if err != nil {
			return libRecords, errors.Wrapf(err, "error parsing ip")
		}
		libRecords = append(libRecords, libdns.Address{
			Name: "@",
			IP:   parsed,
			// net.Resolver does not expose TTL; non-positive values are omitted from update payloads.
			TTL: 0,
		})
	}

	txt, err := r.LookupTXT(ctx, zone)
	if err != nil {
		var dnsErr *net.DNSError
		// Ignore missing dns record
		if !(errors.As(err, &dnsErr) && dnsErr.IsNotFound) {
			return libRecords, errors.Wrapf(err, "error looking up txt")
		}
	}
	for _, t := range txt {
		if t == "" {
			continue
		}
		libRecords = append(libRecords, libdns.TXT{
			Name: "@",
			// net.Resolver does not expose TTL; non-positive values are omitted from update payloads.
			TTL:  0,
			Text: t,
		})
	}

	return libRecords, nil
}

// Set or clear the value of a DNS entry
func (p *Provider) amendRecords(zone string, records []libdns.Record, action Action) ([]libdns.Record, error) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	zone = p.effectiveZone(zone)

	// Make the API request to DNSExit
	// POST Struct, default is JSON content type. No need to set one
	restyClient := p.RestyClient
	if restyClient == nil {
		restyClient = resty.New()
	}

	updateURL := p.UpdateURL
	if updateURL == "" {
		updateURL = "https://api.dnsexit.com/dns/"
	}

	sendForZone := func(zoneToUse string) (*resty.Response, error) {
		var payloadRecords []dnsExitRecord
		for _, record := range records {
			rr := record.RR()

			currentRecord, err := createDnsExitRecord(rr, zoneToUse, action)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Could not convert record for dnsExit: %s", rr))
			}

			payloadRecords = append(payloadRecords, currentRecord)
		}

		payload := dnsExitPayload{Zone: zoneToUse}
		switch action {
		case deleteRecords:
			payload.DeleteRecords = &payloadRecords
		case setRecords:
			fallthrough
		case appendRecords:
			payload.AddRecords = &payloadRecords
		default:
			return nil, errors.New(fmt.Sprintf("Unknown action type: %d", action))
		}

		reqBody, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		if debug {
			fmt.Println("Request Info:")
			fmt.Println("Url:", string(updateURL))
			fmt.Println("Header apikey: <redacted>")
			fmt.Println("Body:", string(reqBody))
		}

		resp, err := restyClient.R().
			SetHeader("Content-Type", "application/json").
			SetHeader("apikey", p.APIKey).
			SetBody(reqBody).
			SetResult(&dnsExitResponse{}).
			SetError(&dnsExitResponse{}).
			Post(updateURL)
		if err != nil {
			return nil, err
		}

		if debug {
			fmt.Println("Response Info:")
			fmt.Printf("Status: %s\n", resp.Status())
			fmt.Printf("Body: %s\n", resp.Body())
		}

		if resp.IsError() {
			return nil, fmt.Errorf("API error: %s", resp.String())
		}

		return resp, nil
	}

	resp, err := sendForZone(zone)
	if err != nil {
		return nil, err
	}

	if !isResposeStatusOK(resp.Body()) {
		msg := responseMessage(resp.Body())
		return nil, errors.New(msg)
	}

	return records, nil
}
func responseMessage(body []byte) string {
	var respJson dnsExitResponse
	_ = json.Unmarshal(body, &respJson)
	return respJson.Message
}

// Convert API response code to human friendly error
func isResposeStatusOK(body []byte) bool {
	var respJson dnsExitResponse
	_ = json.Unmarshal(body, &respJson)
	return respJson.Code == 0
}
