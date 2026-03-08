#!/usr/bin/env bash
# Adds random elements to a HyperLogLog at random intervals
# Usage: ./random_hll.sh [count] [max_interval_seconds]

CLI="valkey-cli -p ${PORT_VALKEY:-6379}"
COUNT=${1:-100}
MAX_INTERVAL=${2:-2}
KEY="hll:live"

# Word components for generating random IDs
PREFIXES=("user" "visitor" "session" "request" "event" "click" "view" "action")
SUFFIXES=("web" "mobile" "api" "app" "bot" "cli")

random_float() {
    local min=$1
    local max=$2
    awk -v min="$min" -v max="$max" -v seed="$RANDOM" 'BEGIN{srand(seed); printf "%.2f", min+rand()*(max-min)}'
}

random_element() {
    local prefix=${PREFIXES[$RANDOM % ${#PREFIXES[@]}]}
    local suffix=${SUFFIXES[$RANDOM % ${#SUFFIXES[@]}]}
    echo "${prefix}_${suffix}_${RANDOM}"
}

echo "Adding $COUNT random elements to $KEY..."
echo "Press Ctrl+C to stop"
echo ""

# Show initial count
initial=$($CLI PFCOUNT "$KEY" 2>/dev/null || echo 0)
echo "Initial cardinality: $initial"
echo ""

for ((i=1; i<=COUNT; i++)); do
    element=$(random_element)

    $CLI PFADD "$KEY" "$element" > /dev/null
    current=$($CLI PFCOUNT "$KEY")

    echo "[$i/$COUNT] Added: $element (cardinality: $current)"

    if [ $i -lt "$COUNT" ]; then
        sleep_time=$(random_float 0.3 "$MAX_INTERVAL")
        sleep "$sleep_time"
    fi
done

echo ""
echo "Done! Final cardinality: $($CLI PFCOUNT "$KEY")"
