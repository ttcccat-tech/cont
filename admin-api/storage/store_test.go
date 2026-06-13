package storage

import (
	"encoding/json"
	"testing"
)

// ComputeConfigDiff is tested via its public interface.
// These tests verify the diff algorithm's correctness.

func TestComputeConfigDiff_EmptyBoth(t *testing.T) {
	prev := `{"services":[],"routes":[],"plugins":[],"consumers":[]}`
	cur := `{"services":[],"routes":[],"plugins":[],"consumers":[]}`
	diff := ComputeConfigDiff(prev, cur)
	var result struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		t.Fatalf("diff result is not valid JSON: %v", err)
	}
	if len(result.Services) != 0 || len(result.Routes) != 0 || len(result.Plugins) != 0 || len(result.Consumers) != 0 {
		t.Errorf("expected empty diff, got %s", diff)
	}
}

func TestComputeConfigDiff_ServiceAdded(t *testing.T) {
	prev := `{"services":[],"routes":[],"plugins":[],"consumers":[]}`
	cur := `{"services":[{"id":"svc-001","name":"test-svc"}],"routes":[],"plugins":[],"consumers":[]}`
	diff := ComputeConfigDiff(prev, cur)
	var result struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		t.Fatalf("diff result is not valid JSON: %v", err)
	}
	if len(result.Services) != 1 {
		t.Fatalf("expected 1 service change, got %d", len(result.Services))
	}
	if result.Services[0]["op"] != "add" {
		t.Errorf("expected op=add, got op=%v", result.Services[0]["op"])
	}
}

func TestComputeConfigDiff_ServiceDeleted(t *testing.T) {
	prev := `{"services":[{"id":"svc-001","name":"test-svc"}],"routes":[],"plugins":[],"consumers":[]}`
	cur := `{"services":[],"routes":[],"plugins":[],"consumers":[]}`
	diff := ComputeConfigDiff(prev, cur)
	var result struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		t.Fatalf("diff result is not valid JSON: %v", err)
	}
	if len(result.Services) != 1 {
		t.Fatalf("expected 1 service change, got %d", len(result.Services))
	}
	if result.Services[0]["op"] != "delete" {
		t.Errorf("expected op=delete, got op=%v", result.Services[0]["op"])
	}
}

func TestComputeConfigDiff_ServiceUpdated(t *testing.T) {
	prev := `{"services":[{"id":"svc-001","name":"old-name","host":"old.example.com"}],"routes":[],"plugins":[],"consumers":[]}`
	cur := `{"services":[{"id":"svc-001","name":"new-name","host":"new.example.com"}],"routes":[],"plugins":[],"consumers":[]}`
	diff := ComputeConfigDiff(prev, cur)
	var result struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		t.Fatalf("diff result is not valid JSON: %v", err)
	}
	if len(result.Services) != 1 {
		t.Fatalf("expected 1 service change, got %d", len(result.Services))
	}
	if result.Services[0]["op"] != "update" {
		t.Errorf("expected op=update, got op=%v", result.Services[0]["op"])
	}
}

func TestComputeConfigDiff_ServiceUnchanged(t *testing.T) {
	prev := `{"services":[{"id":"svc-001","name":"same-name"}],"routes":[],"plugins":[],"consumers":[]}`
	cur := `{"services":[{"id":"svc-001","name":"same-name"}],"routes":[],"plugins":[],"consumers":[]}`
	diff := ComputeConfigDiff(prev, cur)
	var result struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		t.Fatalf("diff result is not valid JSON: %v", err)
	}
	if len(result.Services) != 0 {
		t.Errorf("expected 0 service changes (unchanged), got %d", len(result.Services))
	}
}

func TestComputeConfigDiff_RouteAdded(t *testing.T) {
	prev := `{"services":[],"routes":[],"plugins":[],"consumers":[]}`
	cur := `{"services":[],"routes":[{"id":"r-001","name":"test-route"}],"plugins":[],"consumers":[]}`
	diff := ComputeConfigDiff(prev, cur)
	var result struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		t.Fatalf("diff result is not valid JSON: %v", err)
	}
	if len(result.Routes) != 1 || result.Routes[0]["op"] != "add" {
		t.Errorf("expected 1 route add, got %d changes", len(result.Routes))
	}
}

func TestComputeConfigDiff_RouteDeleted(t *testing.T) {
	prev := `{"services":[],"routes":[{"id":"r-001","name":"test-route"}],"plugins":[],"consumers":[]}`
	cur := `{"services":[],"routes":[],"plugins":[],"consumers":[]}`
	diff := ComputeConfigDiff(prev, cur)
	var result struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		t.Fatalf("diff result is not valid JSON: %v", err)
	}
	if len(result.Routes) != 1 || result.Routes[0]["op"] != "delete" {
		t.Errorf("expected 1 route delete, got %d changes", len(result.Routes))
	}
}

func TestComputeConfigDiff_PluginAdded(t *testing.T) {
	prev := `{"services":[],"routes":[],"plugins":[],"consumers":[]}`
	cur := `{"services":[],"routes":[],"plugins":[{"id":"p-001","name":"rate-limiting"}],"consumers":[]}`
	diff := ComputeConfigDiff(prev, cur)
	var result struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		t.Fatalf("diff result is not valid JSON: %v", err)
	}
	if len(result.Plugins) != 1 || result.Plugins[0]["op"] != "add" {
		t.Errorf("expected 1 plugin add, got %d changes", len(result.Plugins))
	}
}

func TestComputeConfigDiff_ConsumerAdded(t *testing.T) {
	prev := `{"services":[],"routes":[],"plugins":[],"consumers":[]}`
	cur := `{"services":[],"routes":[],"plugins":[],"consumers":[{"id":"c-001","username":"alice"}]}`
	diff := ComputeConfigDiff(prev, cur)
	var result struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		t.Fatalf("diff result is not valid JSON: %v", err)
	}
	if len(result.Consumers) != 1 || result.Consumers[0]["op"] != "add" {
		t.Errorf("expected 1 consumer add, got %d changes", len(result.Consumers))
	}
}

func TestComputeConfigDiff_InvalidJSONPrev(t *testing.T) {
	// Should not panic, should return valid empty-ish diff
	diff := ComputeConfigDiff("not json", `{"services":[],"routes":[],"plugins":[],"consumers":[]}`)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		t.Errorf("should return valid JSON even with invalid input: %v", err)
	}
}

func TestComputeConfigDiff_InvalidJSONCurrent(t *testing.T) {
	diff := ComputeConfigDiff(`{"services":[],"routes":[],"plugins":[],"consumers":[]}`, "not json")
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		t.Errorf("should return valid JSON even with invalid input: %v", err)
	}
}

func TestComputeConfigDiff_MultipleChanges(t *testing.T) {
	prev := `{"services":[{"id":"svc-001","name":"svc"}],"routes":[{"id":"r-001","name":"r"}],"plugins":[],"consumers":[]}`
	cur := `{"services":[{"id":"svc-002","name":"svc-new"}],"routes":[{"id":"r-001","name":"r"}],"plugins":[{"id":"p-001","name":"auth"}],"consumers":[{"id":"c-001","username":"bob"}]}`
	diff := ComputeConfigDiff(prev, cur)
	var result struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(diff), &result); err != nil {
		t.Fatalf("diff result is not valid JSON: %v", err)
	}
	// svc-001 deleted, svc-002 added (net 2 changes)
	if len(result.Services) != 2 {
		t.Errorf("expected 2 service changes, got %d: %v", len(result.Services), result.Services)
	}
	// r-001 unchanged
	if len(result.Routes) != 0 {
		t.Errorf("expected 0 route changes (unchanged), got %d", len(result.Routes))
	}
	// p-001 added
	if len(result.Plugins) != 1 {
		t.Errorf("expected 1 plugin add, got %d", len(result.Plugins))
	}
	// c-001 added
	if len(result.Consumers) != 1 {
		t.Errorf("expected 1 consumer add, got %d", len(result.Consumers))
	}
}
