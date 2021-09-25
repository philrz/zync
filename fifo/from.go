package fifo

import (
	"fmt"
	"time"

	"github.com/brimdata/zed/zbuf"
	"github.com/brimdata/zed/zng"
	"github.com/brimdata/zed/zson"
)

// From  provides a means to sync a kafka topic to a Zed lake in a
// consistent and crash-recoverable fashion.  The data sync'd to the lake
// is assigned a target offset in the lake that may be used to then sync
// the merged lake's data back to another Kafka queue using To.
type From struct {
	zctx  *zson.Context
	src   *Consumer
	dst   *Lake
	batch zbuf.Batch
}

func NewFrom(zctx *zson.Context, dst *Lake, src *Consumer) *From {
	return &From{
		zctx: zctx,
		src:  src,
		dst:  dst,
	}
}

// Make theae configurable
const BatchThresh = 10 * 1024 * 1024
const BatchTimeout = 5 * time.Second

func (f *From) Sync() (int64, int64, error) {
	offset, err := f.dst.NextProducerOffset()
	if err != nil {
		return 0, 0, err
	}
	if offset >= int64(f.src.HighWater())+1 {
		return 0, 0, nil
	}
	// Loop over the records from the kafka consumer and
	// commit a batch at a time to the lake.
	var ncommit, nrec int64
	for {
		batch, err := f.src.Read(BatchThresh, BatchTimeout)
		if err != nil {
			return 0, 0, err
		}
		batchLen := int64(batch.Length())
		if batchLen == 0 {
			break
		}
		batch, err = AdjustOffsets(f.zctx, batch, offset)
		if err != nil {
			return 0, 0, err
		}
		//XXX need to track commitID and use new commit-only-if options
		if _, err := f.dst.LoadBatch(batch); err != nil {
			return 0, 0, err
		}
		offset += batchLen
		nrec += batchLen
		ncommit++
	}
	return ncommit, nrec, nil
}

// AdjustOffsets runs a local Zed program to adjust the kafka offset fields
// for insertion into correct position in the lake and remember the original
// offset
func AdjustOffsets(zctx *zson.Context, batch zbuf.Array, offset int64) (zbuf.Array, error) {
	rec := batch.Index(0)
	kafkaRec, err := batch.Index(0).Access("kafka")
	if err != nil {
		s, err := zson.FormatValue(rec.Value)
		if err != nil {
			// This should not happen.
			err = fmt.Errorf("[ERR! %w]", err)
		}
		// This shouldn't happen since the consumer automatically adds
		// this field.
		return nil, fmt.Errorf("value read from kafka topic missing kafka meta-data field: %s", s)
	}
	// XXX this should be simplified in zed package
	first, err := zng.NewRecord(kafkaRec.Type, kafkaRec.Bytes).AccessInt("offset")
	if err != nil {
		s, err := zson.FormatValue(kafkaRec)
		if err != nil {
			// This should not happen.
			err = fmt.Errorf("[ERR! %w]", err)
		}
		return nil, fmt.Errorf("kafka meta-data field is missing 'offset' field: %s", s)
	}
	query := fmt.Sprintf("kafka.input_offset:=kafka.offset,kafka.offset:=kafka.offset-%d+%d", first, offset)
	return RunLocalQuery(zctx, batch, query)
}
