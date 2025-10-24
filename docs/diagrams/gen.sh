#!/usr/bin/env sh

set -e

OUTPUT_DIR="public/diagrams"
DIAGRAMS_DIR="diagrams"

mkdir -p "$OUTPUT_DIR"

for file in "$DIAGRAMS_DIR"/*.d2; do
    [ -f "$file" ] || continue

    base_name="$(basename "$file" .d2)"
    output_file="$OUTPUT_DIR/${base_name}.svg"

    d2 --pad=0 --theme 101 --dark-theme 200 "$file" "$output_file" &
done

wait
