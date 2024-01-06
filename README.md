# zerolog-sinks-azurebloblogger

To use the package 
```
go get github.com/amitdixit/zerolog-sinks-azurebloblogger

```

```go
package main

import (
	"os"
	"time"

	azBlbLog "github.com/amitdixit/zerolog-sinks-azurebloblogger"
	"github.com/rs/zerolog"
)

type Employee struct {
	Name string
	Age  int
}


func main() {

	logger := initializeZeroLog()

	logger.Info().Msg("Log to Azure Blob Logger")

	emp := Employee{
		Name: "Amit",
		Age:  41,
	}

	logger.Info().Interface("Employee", emp).Msg("Log Employee 1 to Azure Blob Logger")

	emp = Employee{
		Name: "Amit Dixit",
		Age:  41,
	}

	logger.Info().Any("Employee", emp).Msg("Log Employee 2 Azure Blob Logger2")

	time.Sleep(time.Second * 50)

}

func initializeZeroLog() zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	logLevel := zerolog.InfoLevel
	zerolog.SetGlobalLevel(logLevel)

    //FILE_NAME if empty then the logs get stored in the form of {year}/{month}/{day}/{hour}/logs.json
	azBlobWriter, err := azBlbLog.NewAzureBlobWriter("BLOB_CONNECTION_STRING", "CONTAINER_NAME", LOG_SIZE, TIME_INTERVAL, "FILE_NAME")
	if err != nil {
		panic("Problem Configuring Azure Blob Logger")
	}

	multi := zerolog.MultiLevelWriter(os.Stdout, azBlobWriter)

	return zerolog.New(multi).With().Timestamp().Stack().Logger()
}


```