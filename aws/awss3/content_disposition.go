package awss3

import (
	"fmt"
	"net/url"

	"github.com/88labs/go-utils/aws/awss3/options/s3presigned"
)

// ResponseContentDisposition builds the Content-Disposition header value for S3 presigned URLs.
// It encodes the file name using RFC 5987 UTF-8 percent-encoding.
// Returns an empty string for unrecognised ContentDispositionType values.
func ResponseContentDisposition(tp s3presigned.ContentDispositionType, fileName string) string {
	switch tp {
	case s3presigned.ContentDispositionTypeAttachment:
		return fmt.Sprintf(`attachment; filename*=UTF-8''%s`, url.PathEscape(fileName))
	case s3presigned.ContentDispositionTypeInline:
		return fmt.Sprintf(`inline; filename*=UTF-8''%s`, url.PathEscape(fileName))
	default:
		return ""
	}
}
