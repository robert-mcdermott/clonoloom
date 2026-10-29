#!/usr/bin/env bash
# Explicit native Slurm replay task. No eval, scheduler submission or implicit provider consent.
# Example: sbatch --array=0-1 cluster/replay-array.sh /absolute/clonoloom /absolute/bundle /absolute/results
#SBATCH --job-name=clonoloom-replay
#SBATCH --cpus-per-task=2
#SBATCH --mem=2G
#SBATCH --time=00:30:00
#SBATCH --signal=B:TERM@60
#SBATCH --output=clonoloom-%A_%a.log
set -euo pipefail
umask 077
if [[ $# != 3 ]]; then
  echo 'Usage: replay-array.sh ABSOLUTE_BINARY INPUT_BUNDLE OUTPUT_PARENT' >&2
  exit 2
fi
binary=$1
inputs=$2
outputs=$3
index=${SLURM_ARRAY_TASK_ID:-}
job=${SLURM_ARRAY_JOB_ID:-}
if [[ ! $index =~ ^(0|[1-9][0-9]?)$ ]] || (( index >= 64 )) || [[ ! $job =~ ^[0-9]{1,20}$ ]]; then
  echo 'Explicit Slurm array index 0-63 and numeric array job ID are required.' >&2
  exit 2
fi
if [[ $binary != /* || $inputs != /* || $outputs != /* || ! -x $binary || ! -d $inputs || ! -d $outputs ]]; then
  echo 'Use an executable absolute binary path, existing absolute input/output directories.' >&2
  exit 2
fi
task=$(sed -n "$((index + 1))p" "$inputs/tasks.txt")
if [[ ! $task =~ ^task-[0-9]{3}$ ]]; then
  echo 'Array index has no valid explicitly listed task.' >&2
  exit 2
fi
# exec makes the native process receive the batch script's TERM signal directly.
# The site's scheduler must provide enough grace for the documented bounded cancellation.
exec "$binary" run --recipe "$inputs/replay.json" --task "$task" \
  --input "evidence=$inputs/$task.certificate.clz" --out "$outputs/$job-$task"
