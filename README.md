# GitHub Report
Go library to generate reports from GitHub GraphQL API

```go
import (
  "github.com/dsciamma/ghreport"
)

// create a report
report := ghreport.NewActivityReport("AirVantage", "GH_TOKEN", 7)

err := ghreport.Run()
if err != nil {
    log.Fatal(err)
}
// Display the report
```
