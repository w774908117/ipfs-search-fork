package amqp

import (
	"context"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"

	"github.com/ipfs-search/ipfs-search/instr"
)

// Connection wraps an AMQP connection
type Connection struct {
	config *Config
	conn   *amqp.Connection
	*instr.Instrumentation
}

// NewConnection returns new AMQP connection
func NewConnection(ctx context.Context, cfg *Config, amqpConfig *amqp.Config, i *instr.Instrumentation) (*Connection, error) {
	ctx, span := i.Tracer.Start(ctx, "queue.amqp.NewConnection", trace.WithAttributes(label.String("amqp_url", cfg.URL)))
	defer span.End()

	amqpConn, err := amqp.DialConfig(cfg.URL, *amqpConfig)

	if err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return nil, err
	}

	c := &Connection{
		config:          cfg,
		conn:            amqpConn,
		Instrumentation: i,
	}

	blockChan := amqpConn.NotifyBlocked(make(chan amqp.Blocking, 1))
	closeChan := amqpConn.NotifyClose(make(chan *amqp.Error, 1))

	monitorConn := func() {
		ctx, span := i.Tracer.Start(ctx, "queue.amqp.monitorConn", trace.WithAttributes(label.Stringer("connection", c)))
		defer span.End()

		errCnt := 0
		for {
			select {
			case <-ctx.Done():
				err := ctx.Err()
				span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
				return
			case b := <-blockChan:
				if b.Active {
					span.AddEvent(ctx, "amqp-connection-blocked",
						label.String("reason", b.Reason),
					)
					log.Println("AMQP connection blocked")
				} else {
					span.AddEvent(ctx, "amqp-connection-unblocked")
					log.Println("AMQP connection unblocked")
				}
			case err := <-closeChan:
				span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
				log.Printf("AMQP connection lost, attempting reconnect in %s", cfg.ReconnectTime)
				time.Sleep(cfg.ReconnectTime)

				amqpConn, amqpErr := amqp.Dial(cfg.URL)
				if amqpErr != nil {
					if errCnt > cfg.MaxReconnect {
						// TODO: Proper error propagation/recovery
						span.RecordError(ctx, amqpErr, trace.WithErrorStatus(codes.Error))
						panic("Repeated AMQP reconnect errors")
					} else {
						errCnt++
						log.Printf("Error connecting to AMQP: %v", amqpErr)
						span.RecordError(ctx, amqpErr)
					}

				}

				// Set new connection
				c.conn = amqpConn
			}
		}
	}
	go monitorConn()

	return c, nil
}

// Channel creates an AMQP channel
func (c *Connection) channel(ctx context.Context, prefetchCount int) (*Channel, error) {
	ctx, span := c.Tracer.Start(ctx, "queue.amqp.Channel")
	defer span.End()

	// Create channel
	ch, err := c.conn.Channel()
	if err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return nil, err
	}

	// Set Qos
	err = ch.Qos(
		prefetchCount,
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return nil, err
	}

	return &Channel{
		ch:              ch,
		Instrumentation: c.Instrumentation,
		MessageTTL:      c.config.MessageTTL,
	}, nil
}

// NewChannelQueue returns a new queue on a new channel
func (c *Connection) NewChannelQueue(ctx context.Context, name string, prefetchCount int) (*Queue, error) {
	ctx, span := c.Tracer.Start(ctx, "queue.amqp.NewChannelQueue", trace.WithAttributes(label.String("queue", name)))
	defer span.End()

	ch, err := c.channel(ctx, prefetchCount)
	if err != nil {
		return nil, err
	}

	return ch.Queue(ctx, name)
}

func (c *Connection) String() string {
	return c.conn.LocalAddr().String()
}

// Close closes the channel
func (c *Connection) Close() error {
	return c.conn.Close()
}
