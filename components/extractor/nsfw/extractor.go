package nsfw

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"

	"github.com/ipfs-search/ipfs-search/components/extractor"

	"github.com/ipfs-search/ipfs-search/instr"
	t "github.com/ipfs-search/ipfs-search/types"
)

// Extractor extracts metadata using the nsfw-server.
type Extractor struct {
	config *Config
	client *http.Client

	*instr.Instrumentation
}

func (e *Extractor) get(ctx context.Context, url string) (resp *http.Response, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		// Errors here are programming errors.
		panic(fmt.Sprintf("creating request: %s", err))
	}

	return e.client.Do(req)
}

func (e *Extractor) getExtractURL(r *t.AnnotatedResource) string {
	return fmt.Sprintf("%s/classify/%s", e.config.NSFWServerURL, r.ID)
}

// Extract metadata from a (potentially) referenced resource, updating
// Metadata or returning an error.
func (e *Extractor) Extract(ctx context.Context, r *t.AnnotatedResource, m interface{}) error {
	ctx, span := e.Tracer.Start(ctx, "extractor.nsfw_server.Extract")
	defer span.End()

	if r.Protocol != t.IPFSProtocol {
		// This is a programming error until we actually support multiple protocols.
		panic("unsupported protocol")
	}

	// Timeout if extraction hasn't fully completed within this time.
	ctx, cancel := context.WithTimeout(ctx, e.config.RequestTimeout)
	defer cancel()

	if r.Size > uint64(e.config.MaxFileSize) {
		err := fmt.Errorf("%w: %d", extractor.ErrFileTooLarge, r.Size)
		span.RecordError(
			ctx, extractor.ErrFileTooLarge, trace.WithErrorStatus(codes.Error),
			// TODO: Enable after otel upgrade.
			// label.Int64("file.size", r.Size),
		)
		return err
	}

	resp, err := e.get(ctx, e.getExtractURL(r))
	if err != nil {
		err := fmt.Errorf("%w: %v", extractor.ErrRequest, err)
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		err := fmt.Errorf("%w: unexpected status %s", extractor.ErrUnexpectedResponse, resp.Status)
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return err
	}

	// Asynchronously wrap body in {nsfw: }
	reader, writer := io.Pipe()

	go func() {
		io.WriteString(writer, "{\"nfsw\":")
		_, err := io.Copy(writer, resp.Body)
		io.WriteString(writer, "}")
		writer.CloseWithError(err)
	}()

	// Parse resulting JSON
	if err := json.NewDecoder(reader).Decode(m); err != nil {
		err := fmt.Errorf("%w: %v", extractor.ErrUnexpectedResponse, err)
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return err
	}

	log.Printf("Got nsfw metadata metadata for '%v'", r)

	return nil
}

// New returns a new nsfw-server extractor.
func New(config *Config, client *http.Client, instr *instr.Instrumentation) extractor.Extractor {
	return &Extractor{
		config,
		client,
		instr,
	}
}

// Compile-time assurance that implementation satisfies interface.
var _ extractor.Extractor = &Extractor{}
