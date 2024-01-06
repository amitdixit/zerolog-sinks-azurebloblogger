package azurebloblogger

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/streaming"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/appendblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
)

type azureBlobWriter struct {
	containerClient *container.Client
	fileName        string
	logBuffer       chan string
	flushSize       int
	flushInterval   int
	lastFlushTime   time.Time
	flushInProgress bool
	logs            []string
}

var flushTimer *time.Ticker

// Accepts BlobConnectionString,ContainerName,FlushSize,FlushInterval
//
// flushSize : The log item limit to flush to blob
//
// flushInterval: The interval to flush logs to blob in seconds
//
// fileName if empty then the logs get stored in the form of {year}/{month}/{day}/{hour}/logs.json
func NewAzureBlobWriter(connectionString string, containerName string, flushSize int, flushInterval int, fileName string) (*azureBlobWriter, error) {

	azblobClient, err := azblob.NewClientFromConnectionString(connectionString, nil)

	if err != nil {
		return nil, err
	}

	containerClient := azblobClient.ServiceClient().NewContainerClient(containerName)

	azureBlobWriter := &azureBlobWriter{
		containerClient: containerClient,
		logBuffer:       make(chan string, flushSize),
		flushSize:       flushSize,
		flushInterval:   flushInterval,
		lastFlushTime:   time.Now(),
		fileName:        fileName,
		logs:            make([]string, 0, flushSize),
	}

	go azureBlobWriter.startTimer()

	return azureBlobWriter, nil
}

func (w *azureBlobWriter) Write(p []byte) (n int, err error) {

	logEntry := string(p)

	select {
	case w.logBuffer <- logEntry:
	// Successfully queued log entry
	default:
		// Log buffer is full, flush logs directly
		w.flushBufferedLogs()
		w.logBuffer <- logEntry
	}

	return len(p), nil
}

func (w *azureBlobWriter) flushBufferedLogs() {
	if w.flushInProgress {
		return
	}

	w.flushInProgress = true

	go func() {
		defer func() {
			w.flushInProgress = false
		}()

		for {
			select {
			case logEntry := <-w.logBuffer:
				w.logs = append(w.logs, logEntry)
				if len(w.logs) >= w.flushSize {
					w.flushLogs(w.logs)
				}
			default:
				// If no new log entries, flush the existing ones
				if len(w.logs) > 0 {
					w.flushLogs(w.logs)
				}
				return
			}
		}
	}()
}

func (w *azureBlobWriter) startTimer() {
	flushTimer = time.NewTicker(time.Duration(w.flushInterval) * time.Second)
	defer flushTimer.Reset(time.Duration(w.flushInterval) * time.Second)
	for {
		select {
		case <-flushTimer.C:
			// Time to flush logs
			w.flushBufferedLogs()
			flushTimer.Reset(time.Duration(w.flushInterval) * time.Second)
		}
	}
}

func (w *azureBlobWriter) flushLogs(logs []string) {
	ctx := context.TODO()

	if w.fileName == "" {
		t := time.Now()
		w.fileName = fmt.Sprintf("%v/%v/%v/%v/logs.json", t.Year(), t.Month(), t.Day(), t.Hour())
	}

	appendBlob := w.containerClient.NewAppendBlobClient(w.fileName)

	defer func() {
		w.logs = w.logs[:0]
		w.lastFlushTime = time.Now()
	}()

	// Check if blob already exists, create it if not
	props, err := appendBlob.GetProperties(ctx, &blob.GetPropertiesOptions{})

	if err != nil {
		_, err := appendBlob.Create(ctx, &appendblob.CreateOptions{
			HTTPHeaders: &blob.HTTPHeaders{
				BlobContentType:        to.Ptr("application/json"),
				BlobContentDisposition: to.Ptr(fmt.Sprintf("%v.json", w.fileName))},
		})

		if err != nil {
			fmt.Printf("Failed to create blob %v", err)
			return
		}
	}

	offset := props.ContentLength

	var buffer bytes.Buffer
	for _, entry := range logs {
		buffer.WriteString(entry)
	}

	_, err = appendBlob.AppendBlock(ctx, streaming.NopCloser(bytes.NewReader(buffer.Bytes())),
		&appendblob.AppendBlockOptions{

			AppendPositionAccessConditions: &appendblob.AppendPositionAccessConditions{
				AppendPosition: offset,
			},
		})

	if err != nil {
		fmt.Printf("%v", err)
	}
}
