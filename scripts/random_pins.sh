#!/usr/bin/env bash
# Adds random pins near SF coffee shops at random intervals
# Usage: ./random_pins.sh [count] [max_interval_seconds]

CLI="valkey-cli -p ${PORT_VALKEY:-6379}"
COUNT=${1:-50}
MAX_INTERVAL=${2:-5}

# Base coordinates (SF downtown area)
BASE_LAT=37.7866
BASE_LON=-122.4089

# Random business names for variety
BUSINESSES=(
    "Cafe" "Restaurant" "Bar" "Shop" "Store" "Market"
    "Bakery" "Deli" "Boutique" "Studio" "Gallery" "Theater"
    "Gym" "Spa" "Salon" "Clinic" "Office" "Bank"
    "Hotel" "Hostel" "Library" "Museum" "Park" "Plaza"
)

PREFIXES=(
    "The" "Golden" "Urban" "Modern" "Classic" "Vintage"
    "New" "Old" "Grand" "Little" "Big" "Central"
    "Downtown" "Corner" "Local" "Artisan" "Organic" "Fresh"
)

SUFFIXES=(
    "House" "Co" "Place" "Spot" "Point" "Hub"
    "Express" "Plus" "Pro" "Depot" "Station" "Center"
)

# Function to generate random float in range
random_float() {
    local min=$1
    local max=$2
    # Use RANDOM to add entropy to seed
    awk -v min="$min" -v max="$max" -v seed="$RANDOM" 'BEGIN{srand(seed); printf "%.6f", min+rand()*(max-min)}'
}

# Function to generate random name
random_name() {
    local prefix=${PREFIXES[$RANDOM % ${#PREFIXES[@]}]}
    local business=${BUSINESSES[$RANDOM % ${#BUSINESSES[@]}]}
    local use_suffix=$((RANDOM % 2))

    if [ $use_suffix -eq 1 ]; then
        local suffix=${SUFFIXES[$RANDOM % ${#SUFFIXES[@]}]}
        echo "$prefix $business $suffix"
    else
        echo "$prefix $business"
    fi
}

echo "Adding $COUNT random pins near SF coffee shops..."
echo "Press Ctrl+C to stop"
echo ""

for ((i=1; i<=COUNT; i++)); do
    # Generate random offset (roughly 0-500 meters)
    # 1 degree lat ≈ 111km, so 0.005 degrees ≈ 555 meters
    lat_offset=$(random_float -0.005 0.005)
    lon_offset=$(random_float -0.005 0.005)

    lat=$(awk -v base="$BASE_LAT" -v offset="$lat_offset" 'BEGIN{printf "%.6f", base+offset}')
    lon=$(awk -v base="$BASE_LON" -v offset="$lon_offset" 'BEGIN{printf "%.6f", base+offset}')

    name=$(random_name)

    # Add the pin
    if $CLI GEOADD "geo:coffee:downtown" "$lon" "$lat" "$name" > /dev/null; then
        echo "[$i/$COUNT] Added: $name at ($lat, $lon)"
    else
        echo "[$i/$COUNT] Failed to add: $name"
    fi

    # Random sleep interval between 0.5 and MAX_INTERVAL seconds
    if [ $i -lt "$COUNT" ]; then
        sleep_time=$(random_float 0.5 "$MAX_INTERVAL")
        sleep "$sleep_time"
    fi
done

echo ""
echo "Done! Added $COUNT random pins."
echo "Total locations in geo:coffee:downtown: $($CLI ZCARD geo:coffee:downtown)"
