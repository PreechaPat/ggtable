#!/usr/bin/env bash
# Build the real-data test fixture, test_data/pins6/output, with ggorigin.
# pkg/data's TestRealData runs against it (and skips when it is missing).
#
# Genomes: 5 P. insidiosum strains from Table 1 of J. Fungi 2023, 9(2), 169
# (https://doi.org/10.3390/jof9020169), clades I-III, plus P. aphanidermatum
# DAOM BR444 as outgroup. Inputs come from ggprep/data/pythium_sequences.
#
# Usage: EGGNOG_DATA_DIR=/path/to/eggnog scripts/build-test-data.sh
#        (conda env ggorigin active; SKIP_FUNCTION=1 builds without eggNOG)
set -euo pipefail

repo="$(cd "$(dirname "$0")/.." && pwd)"
root="$(dirname "$repo")"
src="${root}/ggprep/data/pythium_sequences/sequences"
strains="${root}/ggprep/data/gene_table_data/list.strains.tab"
out="${repo}/test_data/pins6"
genomes=(CBS57385 ATCC200269 PIS MCC18 SIMI4763 PAPH)

[[ -d "$src" ]] || { echo "ERROR: no genome inputs at $src" >&2; exit 1; }
[[ -f "$strains" ]] || { echo "ERROR: no strain list at $strains" >&2; exit 1; }
if [[ "${SKIP_FUNCTION:-0}" != 1 && ! -s "${EGGNOG_DATA_DIR:-}/eggnog.db" ]]; then
	echo "ERROR: set EGGNOG_DATA_DIR to a folder with eggnog.db, or SKIP_FUNCTION=1" >&2
	exit 1
fi

mkdir -p "${out}/input"
printf "genome_id\tgenome_fullname\n" >"${out}/input/genomes.tsv"
for g in "${genomes[@]}"; do
	[[ -s "${src}/${g}/genes.gff3" ]] || { echo "ERROR: missing ${src}/${g}/genes.gff3" >&2; exit 1; }
	ln -sfn "${src}/${g}" "${out}/input/${g}"
	gawk -F'\t' -v g="$g" '$1 == g { print $1 "\t" $3; exit }' "$strains" >>"${out}/input/genomes.tsv"
done

cd "${root}/ggorigin"
make INPUT_DIR="${out}/input" WORK_DIR="${out}/work" OUTPUT_DIR="${out}/output" \
	EGGNOG_DATA_DIR="${EGGNOG_DATA_DIR:-}" SKIP_FUNCTION="${SKIP_FUNCTION:-0}" THREADS="${THREADS:-$(nproc)}"
echo "Built ${out}/output; run: go test ./pkg/data -run TestRealData -v" >&2
