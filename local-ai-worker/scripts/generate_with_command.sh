#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <prompt_file> <output_path>" >&2
  exit 64
fi

prompt_file="$1"
output_path="$2"

if [[ ! -f "$prompt_file" ]]; then
  echo "prompt file not found: $prompt_file" >&2
  exit 66
fi

if [[ -z "${LOCAL_VIDEO_GENERATOR_COMMAND:-}" ]]; then
  echo "LOCAL_VIDEO_GENERATOR_COMMAND is not configured" >&2
  exit 78
fi

mkdir -p "$(dirname "$output_path")"

# The external command receives the prompt file and desired MP4 output path.
# Example:
# LOCAL_VIDEO_GENERATOR_COMMAND='wan2gp-cli --prompt-file "$1" --output "$2"'
bash -lc "$LOCAL_VIDEO_GENERATOR_COMMAND" bash "$prompt_file" "$output_path"
