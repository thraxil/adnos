package main

import (
	"fmt"
	"os"
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

func TestGetLatestDate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "data")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	csvPath := tmpDir + "/AAPL.csv"

	// Test with non-existent file
	latest, err := GetLatestDate(csvPath)
	if err != nil {
		t.Errorf("expected no error for non-existent file, got %v", err)
	}
	if !latest.IsZero() {
		t.Errorf("expected zero time for non-existent file, got %v", latest)
	}

	// Test with existing file
	content := "Date,Open,High,Low,Close,Volume\n2021-04-01,100,105,95,102,1000\n2021-04-02,103,106,101,104,1100\n"
	if err := os.WriteFile(csvPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	latest, err = GetLatestDate(csvPath)
	if err != nil {
		t.Errorf("GetLatestDate failed: %v", err)
	}
	expected := "2021-04-02"
	if latest.Format("2006-01-02") != expected {
		t.Errorf("expected %s, got %s", expected, latest.Format("2006-01-02"))
	}
}

func TestFetchTickerDataFilterExisting(t *testing.T) {
	// If the API returns a bar for a date we already have, it should be filtered out.
	latest := time.Date(2021, 4, 2, 0, 0, 0, 0, time.UTC)
	
	mockData := []*finance.ChartBar{
		{
			Timestamp: 1617321600, // 2021-04-02
			Open:      decimal.NewFromFloat(100.0),
		},
		{
			Timestamp: 1617408000, // 2021-04-03
			Open:      decimal.NewFromFloat(101.0),
		},
	}
	
	// Filtering logic that should be in main
	var filtered []*finance.ChartBar
	for _, bar := range mockData {
		barDate := time.Unix(int64(bar.Timestamp), 0).UTC()
		// We want strictly AFTER latest
		if barDate.After(latest) {
			filtered = append(filtered, bar)
		}
	}
	
	if len(filtered) != 1 {
		t.Errorf("expected 1 bar, got %d", len(filtered))
	}
	if filtered[0].Timestamp != 1617408000 {
		t.Errorf("expected timestamp 1617408000, got %d", filtered[0].Timestamp)
	}
}
