package awss3_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream/eventstreamapi"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go/eventstream"
	"gotest.tools/v3/assert"

	"github.com/88labs/go-utils/aws/awsconfig"
	"github.com/88labs/go-utils/aws/awss3"
	"github.com/88labs/go-utils/aws/awss3/options/s3selectcsv"
	"github.com/88labs/go-utils/aws/ctxawslocal"
)

func TestSelectCSVAllWithQuotedRecordDelimiter(t *testing.T) {
	// RustFS 1.0.0-rc.6 does not implement AllowQuotedRecordDelimiter.
	// Keep the wrapper's request and event-stream contracts covered independently.
	wantRecords := [][]string{
		{"id", "name", "detail"},
		{"1", "hoge", "あ髙\n社🍣"},
		{"2", "fuga", "い髙\n社🍣"},
		{"3", "piyo", "う髙\n社🍣"},
	}

	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "file LF field LF",
			source: "id,name,detail\n1,hoge,\"あ髙\n社🍣\"\n2,fuga,\"い髙\n社🍣\"\n3,piyo,\"う髙\n社🍣\"",
		},
		{
			name:   "file CRLF field LF",
			source: "id,name,detail\r\n1,hoge,\"あ髙\n社🍣\"\r\n2,fuga,\"い髙\n社🍣\"\r\n3,piyo,\"う髙\n社🍣\"",
		},
		{
			name:   "file LF field CRLF",
			source: "id,name,detail\n1,hoge,\"あ髙\r\n社🍣\"\n2,fuga,\"い髙\r\n社🍣\"\n3,piyo,\"う髙\r\n社🍣\"",
		},
		{
			name:   "file CRLF field CRLF",
			source: "id,name,detail\r\n1,hoge,\"あ髙\r\n社🍣\"\r\n2,fuga,\"い髙\r\n社🍣\"\r\n3,piyo,\"う髙\r\n社🍣\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestBody []byte
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, http.MethodPost, r.Method)
				var err error
				requestBody, err = io.ReadAll(r.Body)
				assert.NilError(t, err)

				w.Header().Set("Content-Type", "application/vnd.amazon.eventstream")
				w.WriteHeader(http.StatusOK)
				responseCSV := strings.ReplaceAll(tt.source, "\r\n", "\n")
				writeSelectRecords(t, w, responseCSV[:len(responseCSV)/2])
				writeSelectRecords(t, w, responseCSV[len(responseCSV)/2:])
				writeSelectEnd(t, w)
			}))
			t.Cleanup(server.Close)

			ctx := ctxawslocal.WithContext(
				context.Background(),
				ctxawslocal.WithS3Endpoint(server.URL),
				ctxawslocal.WithAccessKey("test"),
				ctxawslocal.WithSecretAccessKey("test"),
			)
			client, err := awss3.NewClient(ctx, awsconfig.RegionTokyo)
			assert.NilError(t, err)

			var output bytes.Buffer
			assert.NilError(t, client.SelectCSVAll(ctx, awss3.BucketName("test"), awss3.Key("test.csv"), awss3.SelectCSVAllQuery, &output,
				s3selectcsv.WithCSVInput(types.CSVInput{AllowQuotedRecordDelimiter: aws.Bool(true)}),
			))

			var request struct {
				InputSerialization struct {
					CSV struct {
						AllowQuotedRecordDelimiter *bool `xml:"AllowQuotedRecordDelimiter"`
					} `xml:"CSV"`
				} `xml:"InputSerialization"`
			}
			assert.NilError(t, xml.Unmarshal(requestBody, &request))
			assert.Assert(t, request.InputSerialization.CSV.AllowQuotedRecordDelimiter != nil)
			assert.Equal(t, true, *request.InputSerialization.CSV.AllowQuotedRecordDelimiter)

			records, err := csv.NewReader(strings.NewReader(output.String())).ReadAll()
			assert.NilError(t, err)
			assert.DeepEqual(t, wantRecords, records)
			assert.Equal(t, strings.ReplaceAll(tt.source, "\r\n", "\n"), output.String())
		})
	}
}

func writeSelectRecords(t *testing.T, w http.ResponseWriter, payload string) {
	t.Helper()
	writeSelectEvent(t, w, "Records", []byte(payload))
}

func writeSelectEnd(t *testing.T, w http.ResponseWriter) {
	t.Helper()
	writeSelectEvent(t, w, "End", nil)
}

func writeSelectEvent(t *testing.T, w io.Writer, eventType string, payload []byte) {
	t.Helper()
	err := eventstream.NewEncoder().Encode(w, eventstream.Message{
		Headers: eventstream.Headers{
			{Name: eventstreamapi.MessageTypeHeader, Value: eventstream.StringValue(eventstreamapi.EventMessageType)},
			{Name: eventstreamapi.EventTypeHeader, Value: eventstream.StringValue(eventType)},
		},
		Payload: payload,
	})
	assert.NilError(t, err)
}
