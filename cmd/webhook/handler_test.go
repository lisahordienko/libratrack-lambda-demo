package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

const validBody = `{"event":"loan.overdue","data":{
	"loan_id":" LN-2026-041 ",
	"days_overdue":5,
	"daily_rate":0.50,
	"currency":"usd"}}`

func call(t *testing.T, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, map[string]any) {
	t.Helper()
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler must not return a Go error for client mistakes (would be 502): %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(resp.Body), &out); err != nil {
		t.Fatalf("response body is not JSON: %v (%q)", err, resp.Body)
	}
	return resp, out
}

func TestValidPayloadCalculatesFine(t *testing.T) {
	resp, out := call(t, events.APIGatewayProxyRequest{Body: validBody})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200 (%s)", resp.StatusCode, resp.Body)
	}
	loan := out["loan"].(map[string]any)
	want := map[string]any{
		"loan_id":      "LN-2026-041",
		"days_overdue": float64(5),
		"daily_rate":   0.5,
		"currency":     "USD",
		"fine_amount":  2.5,
	}
	for k, v := range want {
		if loan[k] != v {
			t.Errorf("%s = %v, want %v", k, loan[k], v)
		}
	}
}

func TestBase64Body(t *testing.T) {
	resp, _ := call(t, events.APIGatewayProxyRequest{
		Body:            base64.StdEncoding.EncodeToString([]byte(validBody)),
		IsBase64Encoded: true,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestClientErrorsAre400NotPanicNot502(t *testing.T) {
	cases := map[string]string{
		"malformed json":    `{"event":`,
		"empty body":        ``,
		"unsupported event": `{"event":"loan.returned","data":{}}`,
		"data not object":   `{"event":"loan.overdue","data":"oops"}`,
		"missing data":      `{"event":"loan.overdue"}`,
		"missing fields":    `{"event":"loan.overdue","data":{}}`,
		"invalid days":      `{"event":"loan.overdue","data":{"loan_id":"LN-1","days_overdue":0,"daily_rate":0.5,"currency":"USD"}}`,
		"invalid rate":      `{"event":"loan.overdue","data":{"loan_id":"LN-1","days_overdue":2,"daily_rate":0,"currency":"USD"}}`,
		"invalid currency":  `{"event":"loan.overdue","data":{"loan_id":"LN-1","days_overdue":2,"daily_rate":0.5,"currency":"US"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			resp, out := call(t, events.APIGatewayProxyRequest{Body: body})
			if resp.StatusCode != 400 {
				t.Fatalf("status = %d, want 400 (%s)", resp.StatusCode, resp.Body)
			}
			if out["error"] == nil {
				t.Errorf("response must carry an error message: %s", resp.Body)
			}
		})
	}
}

func TestAllValidationProblemsAreReported(t *testing.T) {
	_, out := call(t, events.APIGatewayProxyRequest{Body: `{"event":"loan.overdue","data":{}}`})
	details := out["details"].([]any)
	if len(details) < 3 {
		t.Fatalf("expected several problems at once, got %v", details)
	}
}
