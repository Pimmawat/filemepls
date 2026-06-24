package http

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// writeDownloadResponse sets the Range-related headers and streams stream
// to the client. Shared by the authenticated file download and the public
// share-redemption download, which need identical semantics.
func writeDownloadResponse(c *gin.Context, stream io.ReadCloser, offset, contentLength, totalSize int64, partial bool, mime string) {
	defer func() { _ = stream.Close() }()

	c.Header("Accept-Ranges", "bytes")
	c.Header("Content-Type", mime)
	c.Header("Content-Length", strconv.FormatInt(contentLength, 10))

	if partial {
		end := offset + contentLength - 1
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, end, totalSize))
		c.Status(http.StatusPartialContent)
	} else {
		c.Status(http.StatusOK)
	}

	if _, err := io.Copy(c.Writer, stream); err != nil {
		log.Printf("download stream copy error: %v", err)
	}
}

// contentDisposition builds an RFC 6266 Content-Disposition header value
// that preserves the original uploaded filename and extension so the file
// opens correctly by association. Falls back to a generic name if the
// original filename is empty.
func contentDisposition(name string, createdAt time.Time) string {
	name = sanitizeHeaderValue(filepath.Base(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "download"
	}

	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		toASCIIFallback(name), url.PathEscape(name))
}

// sanitizeHeaderValue strips control characters (including CR/LF) and
// double quotes so a client-supplied filename can never break out of the
// Content-Disposition header syntax or inject extra header fields.
func sanitizeHeaderValue(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '"' || r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// toASCIIFallback replaces any non-ASCII byte with "_" for the legacy
// filename= parameter; modern clients use the UTF-8 filename* parameter
// instead, which carries the full original name (Thai filenames included).
func toASCIIFallback(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r > 0x7e || r < 0x20 {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
