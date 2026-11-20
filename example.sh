#!/bin/bash

wsm="./zseek2info.wasm"

hdir="./sample.d"
gdir="/guest.d"
bname="izstd.zst"
gpath="${gdir}/${bname}"
hpath="${hdir}/${bname}"

run_wasi() {
	mode=$1
	wazero \
		run \
		-env ENV_ZSTD_PATH="${gpath}" \
		-env ENV_DETAIL_MODE="${mode}" \
		-mount "${hdir}:${gdir}:ro" \
		"${wsm}"
}

input() {
	printf 'helo'
}

genwasi() {
	zstdseekw=$1
	input |
		wazero run "${zstdseekw}" |
		dd of="${hpath}" bs=1048576
}

geninput() {
	echo creating the example zstd file...

	zswasi="${HOME}/go/bin/wasip1_wasm/zstdseek"
	test -f "${zswasi}" && genwasi "${zswasi}"
}

test -f "${hpath}" || geninput

echo simple output
run_wasi simple
echo

echo detailed output
run_wasi detail
echo
