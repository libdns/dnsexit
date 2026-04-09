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
			//TODO - do we care what the TTL is?
			TTL: 8,
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
			TTL:  8,
			Text: t,
		})
	}

	return libRecords, nil
}

// Set or clear the value of a DNS entry
func (p *Provider) amendRecords(zone string, records []libdns.Record, action Action) ([]libdns.Record, error) {

	var payloadRecords []dnsExitRecord
	p.mutex.Lock()
	defer p.mutex.Unlock()

	////////////////////////////////////////////////
	// BUILD PAYLOAD
	////////////////////////////////////////////////
	for _, record := range records {
		rr := record.RR()

		currentRecord, err := createDnsExitRecord(rr, zone, action)

		if err != nil {
			return nil, errors.New(fmt.Sprintf("Could not convert record for dnsExit: %s", rr))
		}

		payloadRecords = append(payloadRecords, currentRecord)
	}

	payload := dnsExitPayload{
		Zone: zone,
	}

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

	////////////////////////////////////////////////
	//SEND PAYLOAD
	////////////////////////////////////////////////

	// Explore response object
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

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

	if debug {
		fmt.Println("Request Info:")
		fmt.Println("Url:", string(updateURL))
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

	//TODO - query the response code and text to determine which updates where successful, and return both records and response text in all cases, rather than just assuming all records for a 0 code and no records for other codes.

	// On any non-zero return code return the API response as the error text.
	if !isResposeStatusOK(resp.Body()) {
		var respJson dnsExitResponse
		_ = json.Unmarshal(resp.Body(), &respJson)
		return nil, errors.New(respJson.Message)
	}

	return records, nil
}

// Convert API response code to human friendly error
func isResposeStatusOK(body []byte) bool {
	var respJson dnsExitResponse
	_ = json.Unmarshal(body, &respJson)
	return respJson.Code == 0
}
