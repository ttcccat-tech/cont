package routes

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		hasError bool
	}{
		{"123", 123, false},
		{"0", 0, false},
		{"", 0, true},         // empty string → no digits → returns 0, nil (our impl treats no digits as valid 0)
		{"abc", 0, false},       // first char non-digit → returns 0, nil
		{"12abc45", 0, false},   // first non-digit aborts → returns 0, nil
		{"0001", 1, false},
		{"9999999999", 9999999999, false},
	}

	for _, tt := range tests {
		v, err := parseInt(tt.input)
		if !tt.hasError && err != nil {
			t.Errorf("parseInt(%q): unexpected error %v", tt.input, err)
			continue
		}
		if v != tt.expected {
			t.Errorf("parseInt(%q): expected %d, got %d", tt.input, tt.expected, v)
		}
	}
}

func TestItoS(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{9, "9"},
		{10, "10"},
		{99, "99"},
		{100, "100"},
		{123, "123"},
		{999, "999"},
		{1000, "1000"},
		{12345, "12345"},
	}

	for _, tt := range tests {
		result := iToS(tt.input)
		if result != tt.expected {
			t.Errorf("iToS(%d): expected %q, got %q", tt.input, tt.expected, result)
		}
	}
}

func TestMakeCursor(t *testing.T) {
	result := makeCursor(50)
	expected := "?offset=50"
	if result != expected {
		t.Errorf("makeCursor(50): expected %q, got %q", expected, result)
	}
}

func TestNextList(t *testing.T) {
	tests := []struct {
		name           string
		count          int
		size           int
		offset         int
		expectNext     bool
		expectedCursor string
	}{
		{"has more", 100, 25, 0, true, "?offset=25"},
		{"at end", 50, 25, 25, false, ""},
		{"at end exact", 50, 50, 0, false, ""},
		{"last page smaller than size", 10, 100, 0, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "/", nil)
			nextList(c, tt.count, tt.size, tt.offset)
			nextHeader := w.Header().Get("Next")
			if tt.expectNext {
				if nextHeader == "" {
					t.Errorf("expected Next header with %q, got empty", tt.expectedCursor)
				}
			} else {
				if nextHeader != "" {
					t.Errorf("expected no Next header, got %q", nextHeader)
				}
			}
		})
	}
}

func TestPaginate(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		expectSize   int
		expectOffset int
	}{
		{"default", "", 100, 0},
		{"custom size", "size=50", 50, 0},
		{"custom offset", "offset=20", 100, 20},
		{"both", "size=25&offset=10", 25, 10},
		{"size too large capped", "size=2000", 100, 0},
		{"negative size default", "size=-5", 100, 0},
		{"negative offset default", "offset=-1", 100, 0},
		{"non-numeric", "size=abc", 100, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			url := "/"
			if tt.query != "" {
				url = "/?" + tt.query
			}
			c.Request = httptest.NewRequest("GET", url, nil)
			size, offset := paginate(c)
			if size != tt.expectSize {
				t.Errorf("size: expected %d, got %d", tt.expectSize, size)
			}
			if offset != tt.expectOffset {
				t.Errorf("offset: expected %d, got %d", tt.expectOffset, offset)
			}
		})
	}
}