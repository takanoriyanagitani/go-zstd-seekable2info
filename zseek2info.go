package zseek2info

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	zs "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
)

type SeekTableWriter func(zs.SeekTable) func(context.Context) error

type KeyVal [2]any

type KeyValWriter func(KeyVal) func(context.Context) error

func (w KeyValWriter) ToSeekTableWriterSimple() SeekTableWriter {
	return func(stab zs.SeekTable) func(context.Context) error {
		return func(ctx context.Context) error {
			e0 := w([2]any{"HasChecksums", stab.HasChecksums()})(ctx)
			e1 := w([2]any{"NumFrames", stab.NumFrames()})(ctx)
			e2 := w([2]any{"Size", stab.Size()})(ctx)
			return errors.Join(
				e0,
				e1,
				e2,
			)
		}
	}
}

type FrameOffsetEntryWriter func(zs.FrameOffsetEntry) func(context.Context) error

func (w KeyValWriter) ToFrameOffsetEntryWriter() FrameOffsetEntryWriter {
	return func(foe zs.FrameOffsetEntry) func(context.Context) error {
		return func(ctx context.Context) error {
			e0 := w([2]any{"ID", foe.ID})(ctx)
			e1 := w([2]any{"CompressedOffset", foe.CompressedOffset})(ctx)
			e2 := w([2]any{"DecompressedOffset", foe.DecompressedOffset})(ctx)
			e3 := w([2]any{"CompressedSize", foe.CompressedSize})(ctx)
			e4 := w([2]any{"DecompressedSize", foe.DecompressedSize})(ctx)
			e5 := w([2]any{"Checksum", foe.Checksum})(ctx)
			return errors.Join(
				e0, e1, e2,
				e3, e4, e5,
			)
		}
	}
}

func (w KeyValWriter) ToSeekTableWriterDetail(
	foeWriter FrameOffsetEntryWriter,
) SeekTableWriter {
	return func(stab zs.SeekTable) func(context.Context) error {
		return func(ctx context.Context) error {
			var framecnt int64 = stab.NumFrames()
			for i := range framecnt {
				foe, ok := stab.EntryByID(i)
				if !ok {
					return nil
				}
				err := foeWriter(foe)(ctx)
				if nil != err {
					return err
				}
			}
			return nil
		}
	}
}

var KvWtrDefault KeyValWriter = func(kval KeyVal) func(context.Context) error {
	return func(ctx context.Context) error {
		var key any = kval[0]
		var val any = kval[1]
		fmt.Printf("%v: %v\n", key, val)
		return nil
	}
}

var SeekTableWriterSimple SeekTableWriter = KvWtrDefault.
	ToSeekTableWriterSimple()

var SeekTableWriterDetail SeekTableWriter = KvWtrDefault.
	ToSeekTableWriterDetail(
		KvWtrDefault.ToFrameOffsetEntryWriter(),
	)

type SeekTableSource func(context.Context) (zs.SeekTable, error)

func (s SeekTableSource) ToSink(
	ctx context.Context,
	sink SeekTableWriter,
) error {
	stab, err := s(ctx)
	if nil != err {
		return err
	}
	return sink(stab)(ctx)
}

type SeekReader struct{ *zs.Reader }

func (r SeekReader) Close() error { return r.Reader.Close() }

func (r SeekReader) ToSource() SeekTableSource {
	return func(_ context.Context) (zs.SeekTable, error) {
		return r.Reader.SeekTable()
	}
}

type IoReader struct{ io.ReadSeeker }

func (i IoReader) ToReader(dec zs.ZSTDDecoder) (*zs.Reader, error) {
	return zs.NewReader(
		i.ReadSeeker,
		dec,
	)
}

type ZstdPath string

func (p ZstdPath) ToFile() (*os.File, error) {
	return os.Open(string(p))
}

func (p ZstdPath) ToSource(dec zs.ZSTDDecoder) SeekTableSource {
	return func(ctx context.Context) (zs.SeekTable, error) {
		var empty zs.SeekTable

		ofile, err := p.ToFile()
		if nil != err {
			return empty, err
		}
		defer ofile.Close() //nolint:errcheck // ignore close error for read

		zrdr, err := IoReader{ReadSeeker: ofile}.ToReader(dec)
		if nil != err {
			return empty, err
		}
		srdr := SeekReader{Reader: zrdr}
		defer srdr.Close() //nolint:errcheck // ignore for now(expecting always nil)

		var src SeekTableSource = srdr.ToSource()
		return src(ctx)
	}
}

func (p ZstdPath) ToTable(
	dec zs.ZSTDDecoder,
) func(context.Context) (zs.SeekTable, error) {
	var src SeekTableSource = p.ToSource(dec)
	return src
}
