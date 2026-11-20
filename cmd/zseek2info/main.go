package main

import (
	"context"
	"log/slog"
	"os"

	zs "github.com/SaveTheRbtz/zstd-seekable-format-go/pkg"
	kz "github.com/klauspost/compress/zstd"
	si "github.com/takanoriyanagitani/go-zstd-seekable2info"
)

var swtrSimple si.SeekTableWriter = si.SeekTableWriterSimple
var swtrDetail si.SeekTableWriter = si.SeekTableWriterDetail

func string2path(s string) si.ZstdPath { return si.ZstdPath(s) }

type io[T any] func(context.Context) (T, error)

func getEnvByKey(key string) io[string] {
	return func(_ context.Context) (string, error) {
		return os.Getenv(key), nil
	}
}

var ioZstdFilename io[string] = getEnvByKey("ENV_ZSTD_PATH")

const modeDefault string = "simple"

func iomode2mapd(orig string) (string, error) {
	var sz int = len(orig)
	switch sz {
	case 0:
		return modeDefault, nil
	default:
		return orig, nil
	}
}

func ioBind[T, U any](
	iofn io[T],
	pure func(T) (U, error),
) io[U] {
	return func(ctx context.Context) (u U, e error) {
		t, err := iofn(ctx)
		if nil != err {
			return u, err
		}
		return pure(t)
	}
}

var ioDetailModeOrig io[string] = getEnvByKey("ENV_DETAIL_MODE")
var ioDetailMode io[string] = ioBind(
	ioDetailModeOrig,
	iomode2mapd,
)

func okfn[T, U any](
	noerr func(T) U,
) func(T) (U, error) {
	return func(t T) (U, error) {
		return noerr(t), nil
	}
}

var ioZstdPath io[si.ZstdPath] = ioBind(ioZstdFilename, okfn(string2path))

func ioAndThen[T, U any](
	i1 io[T],
	f2u func(T) io[U],
) io[U] {
	return func(ctx context.Context) (u U, e error) {
		t, err := i1(ctx)
		if nil != err {
			return u, err
		}

		return f2u(t)(ctx)
	}
}

func dec2table(dec zs.ZSTDDecoder) io[zs.SeekTable] {
	return ioAndThen(
		ioZstdPath,
		func(zpat si.ZstdPath) io[zs.SeekTable] {
			return zpat.ToTable(dec)
		},
	)
}

func ioOr[T any](i io[T], alt T) func(context.Context) T {
	return func(ctx context.Context) T {
		ti, err := i(ctx)
		if nil == err {
			return ti
		}

		return alt
	}
}

type void struct{}

var wtrMap map[string]si.SeekTableWriter = map[string]si.SeekTableWriter{
	"simple": swtrSimple,
	"detail": swtrDetail,
}

func sub(ctx context.Context) error {
	zdec, err := kz.NewReader(nil)
	if nil != err {
		return err
	}
	defer zdec.Close()

	var detailMode string = ioOr(ioDetailMode, modeDefault)(ctx)
	swtr, ok := wtrMap[detailMode]
	if !ok {
		swtr = swtrSimple
	}

	var itab io[zs.SeekTable] = dec2table(zdec)
	var ivoid io[void] = ioAndThen(
		itab,
		func(stab zs.SeekTable) io[void] {
			return func(ctx context.Context) (void, error) {
				err := swtr(stab)(ctx)
				return void{}, err
			}
		},
	)

	_, e := ivoid(ctx)
	return e
}

func main() {
	var ctx context.Context = context.Background()
	err := sub(ctx)
	if nil != err {
		slog.Error(
			"unexpected error",
			"detail", err.Error(),
		)

		var zstdPath string = ioOr(ioZstdFilename, "")(ctx)
		var detailMode string = ioOr(ioDetailMode, modeDefault)(ctx)

		slog.Warn("env info", "ENV_ZSTD_PATH", zstdPath)
		slog.Warn("env info", "ENV_DETAIL_MODE", detailMode)
	}
}
