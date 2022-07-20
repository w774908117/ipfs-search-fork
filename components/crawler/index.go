package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	pool "github.com/libp2p/go-buffer-pool"
	"log"
	"strings"
	"time"

	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"

	"github.com/ipfs-search/ipfs-search/components/extractor"
	"github.com/ipfs-search/ipfs-search/components/index"
	indexTypes "github.com/ipfs-search/ipfs-search/components/index/types"
	t "github.com/ipfs-search/ipfs-search/types"
)

type WantedCID struct {
	Cid      string `json:"cid"`
	FileType string `json:"type"`
}

func makeDocument(r *t.AnnotatedResource) indexTypes.Document {
	now := time.Now().UTC()

	// Strip milliseconds to cater to legacy ES index format.
	// This can be safely removed after the next reindex with _nomillis removed from time format.
	now = now.Truncate(time.Second)

	var references []indexTypes.Reference
	if r.Reference.Parent != nil {
		references = []indexTypes.Reference{
			{
				ParentHash: r.Reference.Parent.ID,
				Name:       r.Reference.Name,
			},
		}
	}

	// Common Document properties
	return indexTypes.Document{
		FirstSeen:  now,
		LastSeen:   now,
		References: references,
		Size:       r.Size,
	}
}

func (c *Crawler) indexInvalid(ctx context.Context, r *t.AnnotatedResource, err error) error {
	// Index unsupported items as invalid.
	return c.indexes.Invalids.Index(ctx, r.ID, &indexTypes.Invalid{
		Error: err.Error(),
	})
}

func (c *Crawler) index(ctx context.Context, r *t.AnnotatedResource) error {
	ctx, span := c.Tracer.Start(ctx, "crawler.index",
		trace.WithAttributes(label.Stringer("type", r.Type)),
	)
	defer span.End()

	var (
		err        error
		index      index.Index
		properties interface{}
	)

	switch r.Type {
	case t.FileType:
		f := &indexTypes.File{
			Document: makeDocument(r),
		}
		err = c.extractor.Extract(ctx, r, f)
		if errors.Is(err, extractor.ErrFileTooLarge) {
			// Interpret files which are too large as invalid resources; prevent repeated attempts.
			span.RecordError(ctx, err)
			err = fmt.Errorf("%w: %v", t.ErrInvalidResource, err)
		}

		index = c.indexes.Files
		properties = f
		// prevent error
		if f.Metadata["Content-Type"] != nil &&
			f.Metadata["Content-Type"].([]interface{}) != nil &&
			len(f.Metadata["Content-Type"].([]interface{})) > 0 {
			typeString := f.Metadata["Content-Type"].([]interface{})[0].(string)
			log.Printf("Got Metadata %s", typeString)
			if strings.Contains(typeString, "text/plain") ||
				strings.Contains(typeString, "json") ||
				strings.Contains(typeString, "html") {
				log.Printf(typeString)
				cidInfo := WantedCID{
					Cid:      r.Resource.ID,
					FileType: typeString,
				}
				buf := pool.GlobalPool.Get(1024 * 512)
				bbuf := bytes.NewBuffer(buf)
				bbuf.Reset()
				w := json.NewEncoder(bbuf)
				if err := w.Encode(cidInfo); err != nil {
					log.Printf("encode %s: unable to marshal %+v to JSON: %s", c.server.remote.String(),
						cidInfo, err)
				}
				err := c.server.writer.WriteMsg(bbuf.Bytes())
				if err != nil {
					log.Printf("Faild to write %s", err)
				}
			}

		}

	case t.DirectoryType:
		d := &indexTypes.Directory{
			Document: makeDocument(r),
		}
		err = c.crawlDir(ctx, r, d)

		index = c.indexes.Directories
		properties = d

	case t.UnsupportedType:
		// Index unsupported items as invalid.
		span.RecordError(ctx, err)
		err = t.ErrUnsupportedType

	case t.PartialType:
		// Index partial (no properties)
		index = c.indexes.Partials
		properties = &indexTypes.Partial{}

	case t.UndefinedType:
		panic("undefined type after Stat call")

	default:
		panic("unexpected type")
	}

	if err != nil {
		if errors.Is(err, t.ErrInvalidResource) {
			log.Printf("Indexing invalid '%v', err: %v", r, err)
			span.RecordError(ctx, err)
			return c.indexInvalid(ctx, r, err)
		}

		return err
	}

	// Index the result
	return index.Index(ctx, r.ID, properties)
}
