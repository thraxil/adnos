# adnos

A simple Go program to fetch historical stock ticker data from Yahoo Finance and save it to CSV files.

## Features
- Fetches daily OHLCV (Open, High, Low, Close, Volume) data.
- **Incremental Updates**: Skips dates already present in existing CSV files and appends new data.
- **Resilience**: Implements exponential backoff retry logic for API rate limits.
- **CLI Interface**: Supports custom start and end dates.

## Prerequisites
- [Go](https://go.dev/doc/install) (1.21 or later recommended)

## Installation

Clone the repository and build the binary using the provided `Makefile`:

```bash
make build
```

This will create an `adnos` executable in the root directory.

## Usage

Create a text file containing one stock ticker per line (e.g., `tickers.txt`):
```text
AAPL
MSFT
GOOGL
```

Run the program by specifying a start date and the path to your tickers file:

```bash
./adnos -start 2023-01-01 tickers.txt
```

### Options
- `-start YYYY-MM-DD`: (Required) The date to begin fetching data from.
- `-end YYYY-MM-DD`: (Optional) The end date for the data range. Defaults to the current date.

### Examples
Fetch data from the beginning of 2023 to now:
```bash
./adnos -start 2023-01-01 tickers.txt
```

Fetch data for a specific window:
```bash
./adnos -start 2023-01-01 -end 2023-12-31 tickers.txt
```

## Output
Data is saved to the `data/` directory, with one CSV file per ticker:
`data/AAPL.csv`
`data/MSFT.csv`
...

The CSV format is: `Date,Open,High,Low,Close,Volume`

## Development

Run tests:
```bash
make test
```

Clean build artifacts and data:
```bash
make clean
```
