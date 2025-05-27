package dnsexit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/libdns/libdns"
	"github.com/stretchr/testify/assert"
)

// mockProvider allows us to override the API endpoint for testing.
type mockProvider struct {
	Provider
	apiURL string
}

// Override the public methods to use the mock's amendRecords
func (p *mockProvider) AppendRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	return p.amendRecords(zone, records, "append")
}
func (p *mockProvider) SetRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	return p.amendRecords(zone, records, "set")
}
func (p *mockProvider) DeleteRecords(ctx context.Context, zone string, records []libdns.Record) ([]libdns.Record, error) {
	return p.amendRecords(zone, records, "delete")
}

func (p *mockProvider) doRequest(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = p.apiURL
	client := &http.Client{}
	return client.Do(req)
}

func (p *mockProvider) amendRecords(zone string, records []libdns.Record, action string) ([]libdns.Record, error) {
	payload := map[string]interface{}{
		"zone":    zone,
		"records": records,
		"action":  action,
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "http://"+p.apiURL+"/dns", bytes.NewReader(body))
	resp, err := p.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return records, nil
}

func setupMockProvider(t *testing.T) (*mockProvider, *map[string]interface{}, *sync.Mutex) {
	var gotPayload map[string]interface{}
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		mu.Lock()
		defer mu.Unlock()
		json.Unmarshal(body, &gotPayload)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	apiURL := server.Listener.Addr().String()
	p := &mockProvider{
		Provider: Provider{APIKey: "dummy"},
		apiURL:   apiURL,
	}
	return p, &gotPayload, &mu
}

func TestProvider_AppendRecords(t *testing.T) {
	ctx := context.Background()
	p, gotPayload, mu := setupMockProvider(t)
	zone := "example.com."
	records := []libdns.Record{
		libdns.Address{
			Name: "ipv4",
			IP:   netip.MustParseAddr("1.2.3.4"),
			TTL:  300,
		},
		libdns.Address{
			Name: "ipv6",
			IP:   netip.MustParseAddr("2001:db8::1"),
			TTL:  400,
		},
		libdns.CNAME{
			Name:   "alias",
			Target: "target.example.com.",
			TTL:    500,
		},
		libdns.MX{
			Name:       "mx",
			Preference: 10,
			Target:     "mail.example.com.",
			TTL:        600,
		},
	}

	_, err := p.AppendRecords(ctx, zone, records)
	assert.NoError(t, err)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, zone, (*gotPayload)["zone"])
	assert.Equal(t, "append", (*gotPayload)["action"])
	recList, ok := (*gotPayload)["records"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, recList, 4)

	// A record
	rec := recList[0].(map[string]interface{})
	assert.Equal(t, "ipv4", rec["Name"])
	assert.Equal(t, "1.2.3.4", rec["IP"])
	assert.InDelta(t, 300, rec["TTL"], 0.1)

	// AAAA record
	rec = recList[1].(map[string]interface{})
	assert.Equal(t, "ipv6", rec["Name"])
	assert.Equal(t, "2001:db8::1", rec["IP"])
	assert.InDelta(t, 400, rec["TTL"], 0.1)

	// CNAME record
	rec = recList[2].(map[string]interface{})
	assert.Equal(t, "alias", rec["Name"])
	assert.Equal(t, "target.example.com.", rec["Target"])
	assert.InDelta(t, 500, rec["TTL"], 0.1)

	// MX record
	rec = recList[3].(map[string]interface{})
	assert.Equal(t, "mx", rec["Name"])
	assert.Equal(t, "mail.example.com.", rec["Target"])
	assert.Equal(t, float64(10), rec["Preference"])
	assert.InDelta(t, 600, rec["TTL"], 0.1)
}

func TestProvider_SetRecords(t *testing.T) {
	ctx := context.Background()
	p, gotPayload, mu := setupMockProvider(t)
	zone := "example.com."
	records := []libdns.Record{
		libdns.Address{
			Name: "ipv4",
			IP:   netip.MustParseAddr("1.2.3.4"),
			TTL:  300,
		},
		libdns.Address{
			Name: "ipv6",
			IP:   netip.MustParseAddr("2001:db8::1"),
			TTL:  400,
		},
		libdns.CNAME{
			Name:   "alias",
			Target: "target.example.com.",
			TTL:    500,
		},
		libdns.MX{
			Name:       "mx",
			Preference: 10,
			Target:     "mail.example.com.",
			TTL:        600,
		},
	}

	_, err := p.SetRecords(ctx, zone, records)
	assert.NoError(t, err)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, zone, (*gotPayload)["zone"])
	assert.Equal(t, "set", (*gotPayload)["action"])
	recList, ok := (*gotPayload)["records"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, recList, 4)

	// A record
	rec := recList[0].(map[string]interface{})
	assert.Equal(t, "ipv4", rec["Name"])
	assert.Equal(t, "1.2.3.4", rec["IP"])
	assert.InDelta(t, 300, rec["TTL"], 0.1)

	// AAAA record
	rec = recList[1].(map[string]interface{})
	assert.Equal(t, "ipv6", rec["Name"])
	assert.Equal(t, "2001:db8::1", rec["IP"])
	assert.InDelta(t, 400, rec["TTL"], 0.1)

	// CNAME record
	rec = recList[2].(map[string]interface{})
	assert.Equal(t, "alias", rec["Name"])
	assert.Equal(t, "target.example.com.", rec["Target"])
	assert.InDelta(t, 500, rec["TTL"], 0.1)

	// MX record
	rec = recList[3].(map[string]interface{})
	assert.Equal(t, "mx", rec["Name"])
	assert.Equal(t, "mail.example.com.", rec["Target"])
	assert.Equal(t, float64(10), rec["Preference"])
	assert.InDelta(t, 600, rec["TTL"], 0.1)
}
func TestProvider_DeleteRecords(t *testing.T) {
	ctx := context.Background()
	p, gotPayload, mu := setupMockProvider(t)
	zone := "example.com."
	records := []libdns.Record{
		libdns.Address{
			Name: "ipv4",
			IP:   netip.MustParseAddr("1.2.3.4"),
			TTL:  300,
		},
		libdns.Address{
			Name: "ipv6",
			IP:   netip.MustParseAddr("2001:db8::1"),
			TTL:  400,
		},
		libdns.CNAME{
			Name:   "alias",
			Target: "target.example.com.",
			TTL:    500,
		},
		libdns.MX{
			Name:       "mx",
			Preference: 10,
			Target:     "mail.example.com.",
			TTL:        600,
		},
	}

	_, err := p.DeleteRecords(ctx, zone, records)
	assert.NoError(t, err)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, zone, (*gotPayload)["zone"])
	assert.Equal(t, "delete", (*gotPayload)["action"])
	recList, ok := (*gotPayload)["records"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, recList, 4)

	// A record
	rec := recList[0].(map[string]interface{})
	assert.Equal(t, "ipv4", rec["Name"])
	assert.Equal(t, "1.2.3.4", rec["IP"])
	assert.InDelta(t, 300, rec["TTL"], 0.1)

	// AAAA record
	rec = recList[1].(map[string]interface{})
	assert.Equal(t, "ipv6", rec["Name"])
	assert.Equal(t, "2001:db8::1", rec["IP"])
	assert.InDelta(t, 400, rec["TTL"], 0.1)

	// CNAME record
	rec = recList[2].(map[string]interface{})
	assert.Equal(t, "alias", rec["Name"])
	assert.Equal(t, "target.example.com.", rec["Target"])
	assert.InDelta(t, 500, rec["TTL"], 0.1)

	// MX record
	rec = recList[3].(map[string]interface{})
	assert.Equal(t, "mx", rec["Name"])
	assert.Equal(t, "mail.example.com.", rec["Target"])
	assert.Equal(t, float64(10), rec["Preference"])
	assert.InDelta(t, 600, rec["TTL"], 0.1)
}

func TestAppendRecords_JSONPayloadAndErrorHandling(t *testing.T) {
	type apiResponse struct {
		ReplyCode    int    `json:"reply_code"`
		ReplyMessage string `json:"reply_message"`
	}

	testCases := []struct {
		name        string
		replyCode   int
		replyMsg    string
		expectError bool
	}{
		{"Success", 0, "Success", false},
		{"Some execution problems", 1, "Some execution problems", true},
		{"API Key Authentication Error", 2, "API Key Authentication Error", true},
		{"Missing Required Definitions", 3, "Missing Required Definitions", true},
		{"JSON Data Syntax Error", 4, "JSON Data Syntax Error", true},
		{"JSON Defined Record Type not Supported", 5, "JSON Defined Record Type not Supported", true},
		{"System Error", 6, "System Error", true},
		{"Error getting post data", 7, "Error getting post data", true},
	}

	var currentResp apiResponse

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		js, _ := json.Marshal(currentResp)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(js)
	}))
	defer ts.Close()

	restyClient := resty.New()
	restyClient.SetBaseURL(ts.URL)

	provider := &Provider{
		APIKey:      "dummy",
		RestyClient: restyClient,
		UpdateURL:   ts.URL,
	}

	records := []libdns.Record{
		libdns.Address{
			Name: "test",
			IP:   netip.MustParseAddr("1.2.3.4"),
			TTL:  300,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			currentResp = apiResponse{
				ReplyCode:    tc.replyCode,
				ReplyMessage: tc.replyMsg,
			}
			_, err := provider.AppendRecords(nil, "example.com", records)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error for reply code %d, got nil", tc.replyCode)
				} else if err.Error() != tc.replyMsg {
					t.Errorf("expected error message '%s', got '%s'", tc.replyMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("expected no error for reply code %d, got %v", tc.replyCode, err)
				}
			}
		})
	}
}
