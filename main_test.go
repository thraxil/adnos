package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/piquette/finance-go"
	"github.com/shopspring/decimal"
)

func TestReadTickers(t *testing.T) {
	content := "AAPL\nMSFT\nGOOGL\n"
	tmpfile, err := os.CreateTemp("", "tickers.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tmpfile.Close(); err != nil {
		t.Fatal(err)
	}

	expected := []string{"AAPL", "MSFT", "GOOGL"}
	actual, err := ReadTickers(tmpfile.Name())
	if err != nil {
		t.Errorf("ReadTickers failed: %v", err)
	}

	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("expected %v, got %v", expected, actual)
	}
}

type MockDownloader struct {
	Data []*finance.ChartBar
	Err  error
}

func (m *MockDownloader) GetHistorical(ticker string, start, end time.Time) ([]*finance.ChartBar, error) {
	return m.Data, m.Err
}

func TestFetchTickerData(t *testing.T) {
	mockData := []*finance.ChartBar{
		{
			Timestamp: 1617235200,
			Open:      decimal.NewFromFloat(100.0),
			High:      decimal.NewFromFloat(105.0),
			Low:       decimal.NewFromFloat(95.0),
			Close:     decimal.NewFromFloat(102.0),
			Volume:    1000,
		},
	}
	mock := &MockDownloader{Data: mockData}

	start := time.Date(2021, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2021, 4, 2, 0, 0, 0, 0, time.UTC)

	data, err := FetchTickerData(mock, "AAPL", start, end)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(data, mockData) {
		t.Errorf("expected %v, got %v", mockData, data)
	}
}

type FailingMockDownloader struct {
	FailCount int
	Data      []*finance.ChartBar
	Err       error
	attempts  int
}

func (m *FailingMockDownloader) GetHistorical(ticker string, start, end time.Time) ([]*finance.ChartBar, error) {
	m.attempts++
	if m.attempts <= m.FailCount {
		return nil, m.Err
	}
	return m.Data, nil
}

func TestFetchTickerDataWithRetry(t *testing.T) {
	mockData := []*finance.ChartBar{
		{
			Timestamp: 1617235200,
			Open:      decimal.NewFromFloat(100.0),
			High:      decimal.NewFromFloat(105.0),
			Low:       decimal.NewFromFloat(95.0),
			Close:     decimal.NewFromFloat(102.0),
			Volume:    1000,
		},
	}
	mock := &FailingMockDownloader{
		FailCount: 2,
		Data:      mockData,
		Err:       fmt.Errorf("rate limit"),
	}

	start := time.Date(2021, 4, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2021, 4, 2, 0, 0, 0, 0, time.UTC)

	// Use a short delay for testing
	data, err := FetchTickerDataWithRetry(mock, "AAPL", start, end, 5, 1*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	if mock.attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", mock.attempts)
	}

	if !reflect.DeepEqual(data, mockData) {
		t.Errorf("expected %v, got %v", mockData, data)
	}
}

func TestWriteToCSV(t *testing.T) {
	ticker := "AAPL"
	data := []*finance.ChartBar{
		{
			Timestamp: 1617235200,
			Open:      decimal.NewFromFloat(100.0),
			High:      decimal.NewFromFloat(105.0),
			Low:       decimal.NewFromFloat(95.0),
			Close:     decimal.NewFromFloat(102.0),
			Volume:    1000,
		},
	}

	// Create a temporary data directory
	tmpDir, err := os.MkdirTemp("", "data")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	err = WriteToCSV(tmpDir, ticker, data)
	if err != nil {
		t.Fatal(err)
	}

	expectedFile := tmpDir + "/AAPL.csv"
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", expectedFile)
	}

	content, err := os.ReadFile(expectedFile)
	if err != nil {
		t.Fatal(err)
	}

	expectedContent := "Date,Open,High,Low,Close,Volume\n2021-04-01,100,105,95,102,1000\n"
	if string(content) != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, string(content))
	}
}

func TestParseDates(t *testing.T) {
	// Test valid dates
	startStr := "2021-01-01"
	endStr := "2021-01-02"
	start, end, err := ParseDates(startStr, endStr)
	if err != nil {
		t.Fatal(err)
	}
	if start.Format("2006-01-02") != "2021-01-01" {
		t.Errorf("expected start 2021-01-01, got %v", start)
	}
	if end.Format("2006-01-02") != "2021-01-02" {
		t.Errorf("expected end 2021-01-02, got %v", end)
	}

	// Test default end date
	start, end, err = ParseDates(startStr, "")
	if err != nil {
		t.Fatal(err)
	}
	if start.Format("2006-01-02") != "2021-01-01" {
		t.Errorf("expected start 2021-01-01, got %v", start)
	}
	now := time.Now().UTC().Format("2006-01-02")
	if end.Format("2006-01-02") != now {
		t.Errorf("expected end %s, got %v", now, end)
	}

	// Test invalid date
	_, _, err = ParseDates("invalid", "")
	if err == nil {
		t.Error("expected error for invalid date, got nil")
	}
}

func TestDedupeFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dedupe_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "TEST.csv")
	content := "Date,Open,High,Low,Close,Volume\n" +
		"2021-04-01,100,105,95,102,1000\n" +
		"2021-04-02,103,106,101,104,1100\n" +
		"2021-04-02,103,106,101,104,1100\n" + // Duplicate
		"2021-04-03,105,108,104,107,1200\n"

	if err := os.WriteFile(csvPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	if err := dedupeFile(csvPath); err != nil {
		t.Errorf("dedupeFile failed: %v", err)
	}

	newContent, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatal(err)
	}

	expectedContent := "Date,Open,High,Low,Close,Volume\n" +
		"2021-04-01,100,105,95,102,1000\n" +
		"2021-04-02,103,106,101,104,1100\n" +
		"2021-04-03,105,108,104,107,1200\n"

	if string(newContent) != expectedContent {
		t.Errorf("expected content:\n%q\ngot:\n%q", expectedContent, string(newContent))
	}
}

func TestFetchTickerDataFilterExisting(t *testing.T) {
	// If the API returns a bar for a date we already have, it should be filtered out.
	latest := time.Date(2021, 4, 2, 0, 0, 0, 0, time.UTC)
	
	mockData := []*finance.ChartBar{
		{
			Timestamp: 1617321600 + 3600, // 2021-04-02 01:00:00 UTC
			Open:      decimal.NewFromFloat(100.0),
		},
		{
			Timestamp: 1617408000 + 3600, // 2021-04-03 01:00:00 UTC
			Open:      decimal.NewFromFloat(101.0),
		},
	}
	
	// Filtering logic that is in main
	var filtered []*finance.ChartBar
	for _, bar := range mockData {
		barDate := time.Unix(int64(bar.Timestamp), 0).UTC()
		truncatedBarDate := time.Date(barDate.Year(), barDate.Month(), barDate.Day(), 0, 0, 0, 0, time.UTC)
		// We want strictly AFTER latest
		if !latest.IsZero() && (truncatedBarDate.Before(latest) || truncatedBarDate.Equal(latest)) {
			continue
		}
		filtered = append(filtered, bar)
	}
	
	if len(filtered) != 1 {
		t.Errorf("expected 1 bar, got %d", len(filtered))
	}
	if filtered[0].Timestamp != 1617408000 + 3600 {
		t.Errorf("expected timestamp 1617411600, got %d", filtered[0].Timestamp)
	}
}
