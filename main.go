package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/piquette/finance-go"
	"github.com/piquette/finance-go/chart"
	"github.com/piquette/finance-go/datetime"
)

type Downloader interface {
	GetHistorical(ticker string, start, end time.Time) ([]*finance.ChartBar, error)
}

type UserAgentTransport struct {
	Base http.RoundTripper
}

func (u *UserAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	return u.Base.RoundTrip(req)
}

type YahooDownloader struct {
	Client *http.Client
}

func NewYahooDownloader() *YahooDownloader {
	return &YahooDownloader{
		Client: &http.Client{
			Transport: &UserAgentTransport{
				Base: http.DefaultTransport,
			},
		},
	}
}

func (y *YahooDownloader) GetHistorical(ticker string, start, end time.Time) ([]*finance.ChartBar, error) {
	p := &chart.Params{
		Symbol:   ticker,
		Start:    datetime.New(&start),
		End:      datetime.New(&end),
		Interval: datetime.OneDay,
	}

	chartClient := chart.Client{
		B: finance.NewBackends(y.Client).YFin,
	}

	iter := chartClient.Get(p)
	var data []*finance.ChartBar
	for iter.Next() {
		data = append(data, iter.Bar())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return data, nil
}

func FetchTickerData(d Downloader, ticker string, start, end time.Time) ([]*finance.ChartBar, error) {
	return FetchTickerDataWithRetry(d, ticker, start, end, 5, 1*time.Second)
}

func FetchTickerDataWithRetry(d Downloader, ticker string, start, end time.Time, maxRetries int, baseDelay time.Duration) ([]*finance.ChartBar, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		data, err := d.GetHistorical(ticker, start, end)
		if err == nil {
			return data, nil
		}
		lastErr = err

		delay := baseDelay * (1 << i)
		fmt.Printf("Error fetching %s: %v, retrying in %v (attempt %d/%d)...\n", ticker, err, delay, i+1, maxRetries)
		time.Sleep(delay)
	}
	return nil, lastErr
}

func GetLatestDate(csvPath string) (time.Time, error) {
	file, err := os.Open(csvPath)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return time.Time{}, err
	}

	if len(records) <= 1 {
		return time.Time{}, nil
	}

	lastRecord := records[len(records)-1]
	return time.Parse("2006-01-02", lastRecord[0])
}

func WriteToCSV(dir, ticker string, data []*finance.ChartBar) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s/%s.csv", dir, ticker)
	fileExist := false
	if _, err := os.Stat(filename); err == nil {
		fileExist = true
	}

	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if !fileExist {
		writer.Write([]string{"Date", "Open", "High", "Low", "Close", "Volume"})
	}

	for _, bar := range data {
		date := time.Unix(int64(bar.Timestamp), 0).UTC().Format("2006-01-02")
		writer.Write([]string{
			date,
			bar.Open.String(),
			bar.High.String(),
			bar.Low.String(),
			bar.Close.String(),
			fmt.Sprintf("%d", bar.Volume),
		})
	}

	return writer.Error()
}

func ParseDates(startStr, endStr string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	var end time.Time
	if endStr == "" {
		end = time.Now().UTC()
	} else {
		end, err = time.Parse("2006-01-02", endStr)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}

	return start, end, nil
}

func ReadTickers(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var tickers []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			tickers = append(tickers, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return tickers, nil
}

func main() {
	var startStr, endStr string
	flag.StringVar(&startStr, "start", "", "Start date (YYYY-MM-DD)")
	flag.StringVar(&endStr, "end", "", "End date (YYYY-MM-DD, optional, defaults to now)")
	flag.Parse()

	if startStr == "" {
		fmt.Println("Usage: program -start YYYY-MM-DD [-end YYYY-MM-DD] <tickers_file>")
		os.Exit(1)
	}

	tickersFile := flag.Arg(0)
	if tickersFile == "" {
		fmt.Println("Error: tickers file is required")
		os.Exit(1)
	}

	start, end, err := ParseDates(startStr, endStr)
	if err != nil {
		fmt.Printf("Error parsing dates: %v\n", err)
		os.Exit(1)
	}

	tickers, err := ReadTickers(tickersFile)
	if err != nil {
		fmt.Printf("Error reading tickers: %v\n", err)
		os.Exit(1)
	}

	d := NewYahooDownloader()
	for _, ticker := range tickers {
		fmt.Printf("Processing %s...\n", ticker)
		csvPath := fmt.Sprintf("data/%s.csv", ticker)
		latest, err := GetLatestDate(csvPath)
		if err != nil {
			fmt.Printf("Error checking existing data for %s: %v\n", ticker, err)
			continue
		}

		currentStart := start
		if !latest.IsZero() {
			if latest.After(end) || latest.Equal(end) {
				fmt.Printf("Data for %s is already up to date (latest: %s)\n", ticker, latest.Format("2006-01-02"))
				continue
			}
			// Start from the day after the latest date in the file
			currentStart = latest.AddDate(0, 0, 1)
		}

		if currentStart.After(end) {
			fmt.Printf("Data for %s is already up to date (latest: %s)\n", ticker, latest.Format("2006-01-02"))
			continue
		}

		fmt.Printf("Fetching data for %s from %s to %s...\n", ticker, currentStart.Format("2006-01-02"), end.Format("2006-01-02"))
		data, err := FetchTickerData(d, ticker, currentStart, end)
		if err != nil {
			fmt.Printf("Error fetching data for %s: %v\n", ticker, err)
			continue
		}

		var newData []*finance.ChartBar
		for _, bar := range data {
			barDate := time.Unix(int64(bar.Timestamp), 0).UTC()
			// Only include if it is strictly after the latest date in our file.
			// Dates from Parse("2006-01-02") are midnight UTC.
			if !latest.IsZero() && (barDate.Before(latest) || barDate.Equal(latest)) {
				continue
			}
			newData = append(newData, bar)
		}

		if len(newData) == 0 {
			fmt.Printf("No new data found for %s after filtering\n", ticker)
			continue
		}

		err = WriteToCSV("data", ticker, newData)
		if err != nil {
			fmt.Printf("Error writing CSV for %s: %v\n", ticker, err)
			continue
		}
	}
}
