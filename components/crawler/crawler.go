// Package crawler is grouped around the Crawler component, crawling and indexing content from an AnnotatedResource.
package crawler

import (
	"context"
	"errors"
	"github.com/libp2p/go-msgio"
	"log"
	"net"
	"os"
	"time"

	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"

	"github.com/ipfs-search/ipfs-search/components/extractor"
	"github.com/ipfs-search/ipfs-search/components/protocol"

	"github.com/ipfs-search/ipfs-search/instr"
	t "github.com/ipfs-search/ipfs-search/types"
)

// Crawler allows crawling of resources.
type Crawler struct {
	config    *Config
	indexes   *Indexes
	queues    *Queues
	protocol  protocol.Protocol
	extractor extractor.Extractor

	*instr.Instrumentation
	server      *tcpServer
	videoServer *tcpServer
}

// tcpserver which the client subscribed to
type tcpServer struct {
	// The address of the client.
	remote net.TCPAddr

	// The TCP connection.
	conn net.Conn

	// A 4-byte, big-endian frame-delimited writer.
	writer msgio.WriteCloser

	// A 4-byte, big-endian frame-delimited reader.
	reader msgio.ReadCloser
}

func isSupportedType(rType t.ResourceType) bool {
	switch rType {
	case t.UndefinedType, t.FileType, t.DirectoryType:
		return true
	default:
		return false
	}
}

// Crawl updates existing or crawls new resources, extracting metadata where applicable.
func (c *Crawler) Crawl(ctx context.Context, r *t.AnnotatedResource) error {
	ctx, span := c.Tracer.Start(ctx, "crawler.Crawl",
		trace.WithAttributes(label.String("cid", r.ID)),
	)
	defer span.End()

	var err error

	if r.Protocol == t.InvalidProtocol {
		// Sending items with an invalid protocol to Crawl() is a programming error and
		// should never happen.
		panic("invalid protocol")
	}

	if !isSupportedType(r.Type) {
		// Calling crawler with unsupported types is undefined behaviour.
		panic("invalid type for crawler")
	}

	exists, err := c.updateMaybeExisting(ctx, r)
	if err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return err
	}

	if exists {
		log.Printf("Not updating existing resource %v", r)
		span.AddEvent(ctx, "Not updating existing resource")
		return nil
	}

	if err := c.ensureType(ctx, r); err != nil {
		if errors.Is(err, t.ErrInvalidResource) {
			// Resource is invalid, index as such, throwing away ErrInvalidResource in favor of the result of indexing operation.
			log.Printf("Indexing invalid resource %v", r)
			span.AddEvent(ctx, "Indexing invalid resource")

			err = c.indexInvalid(ctx, r, err)
		}

		// Errors from ensureType imply that no type could be found, hence we can't index.
		if err != nil {
			span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		}
		return err
	}

	log.Printf("Indexing new item %v", r)
	err = c.index(ctx, r)
	if err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
	}
	return err
}
func establishConnection(url string) (net.Conn, net.TCPAddr) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", url)
	if err != nil {
		log.Printf("Error at resolving tcp address %s", url)
	}
	tryMax := 20
	for try := 0; try < tryMax; {
		conn, err := net.DialTCP("tcp", nil, tcpAddr)
		if err != nil {
			log.Printf("Error at dialing tcp address %s", url)
			log.Printf("Retry %d/%d with sleeping 1s", try, tryMax)
			time.Sleep(time.Second * 1)
			try += 1
			continue
		}
		return conn, *tcpAddr
	}
	log.Printf("Failed Dailing Injection Server")
	os.Exit(1)
	return nil, *tcpAddr
}

// New instantiates a Crawler.
func New(config *Config, indexes *Indexes, queues *Queues, protocol protocol.Protocol, extractor extractor.Extractor, i *instr.Instrumentation) *Crawler {
	c, tcpAddr := establishConnection(config.ServerURL)
	server := &tcpServer{
		remote: tcpAddr,
		conn:   c,
		writer: msgio.NewWriter(c),
		reader: msgio.NewReader(c),
	}
	videoC, videoTcpAddr := establishConnection(config.VideoServerURL)
	videoServer := &tcpServer{
		remote: videoTcpAddr,
		conn:   videoC,
		writer: msgio.NewWriter(videoC),
		reader: msgio.NewReader(videoC),
	}
	return &Crawler{
		config,
		indexes,
		queues,
		protocol,
		extractor,
		i,
		server,
		videoServer,
	}
}

func (c *Crawler) ensureType(ctx context.Context, r *t.AnnotatedResource) error {
	ctx, span := c.Tracer.Start(ctx, "crawler.ensureType")
	defer span.End()

	var err error

	if r.Type == t.UndefinedType {
		ctx, cancel := context.WithTimeout(ctx, c.config.StatTimeout)
		defer cancel()

		err = c.protocol.Stat(ctx, r)
		if err != nil {
			span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		}
	}

	return err
}
