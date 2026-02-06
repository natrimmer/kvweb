#!/usr/bin/env bash

# Parse arguments
TYPE="${1:-all}"  # Default to "all" if no argument provided

if [[ "$TYPE" != "all" && "$TYPE" != "string" && "$TYPE" != "hash" && "$TYPE" != "list" && "$TYPE" != "set" && "$TYPE" != "zset" && "$TYPE" != "geo" && "$TYPE" != "stream" && "$TYPE" != "hll" && "$TYPE" != "ttl" ]]; then
    echo "Usage: $0 [TYPE]"
    echo ""
    echo "TYPE can be one of:"
    echo "  all     - Seed all data types (default)"
    echo "  string  - String values only"
    echo "  hash    - Hash values only"
    echo "  list    - List values only"
    echo "  set     - Set values only"
    echo "  zset    - Sorted set values only"
    echo "  geo     - Geospatial values only"
    echo "  stream  - Stream values only"
    echo "  hll     - HyperLogLog values only"
    echo "  ttl     - Keys with TTL only"
    exit 1
fi

echo "Seeding valkey with sample data (type: $TYPE)..."
CLI="valkey-cli -p $PORT_VALKEY"

# ===================
# STRING
# ===================

if [[ "$TYPE" == "all" || "$TYPE" == "string" ]]; then
    # Short strings
    $CLI SET "string:short:empty" ""
    $CLI SET "string:short:char" "x"
    $CLI SET "string:short:word" "hello"
    $CLI SET "string:short:number" "42"
    $CLI SET "string:short:flag" "true"

    # Long strings
    $CLI SET "string:long:paragraph" "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."
    $CLI SET "string:long:multiline" "Line 1: Introduction to the system architecture
Line 2: The backend service handles API requests
Line 3: Data is persisted in the key-value store
Line 4: The frontend renders the user interface
Line 5: Authentication uses JWT tokens
Line 6: Rate limiting prevents abuse
Line 7: Caching improves response times
Line 8: Logs are aggregated for monitoring
Line 9: Metrics track system health
Line 10: Alerts notify on-call engineers"
    $CLI SET "string:long:code" "function processData(input) {\n  const validated = validateInput(input);\n  if (!validated.success) {\n    throw new Error(validated.error);\n  }\n  const transformed = transform(validated.data);\n  const result = aggregate(transformed);\n  return {\n    status: 'complete',\n    data: result,\n    timestamp: Date.now()\n  };\n}"

    # JSON strings
    $CLI SET "string:json:simple" '{"name":"test","value":123}'
    $CLI SET "string:json:nested" '{"user":{"id":1,"profile":{"name":"Alice","settings":{"theme":"dark","notifications":{"email":true,"push":false}}}}}'
    $CLI SET "string:json:array" '[{"id":1,"name":"first"},{"id":2,"name":"second"},{"id":3,"name":"third"}]'
    $CLI SET "string:json:complex" '{"apiVersion":"v1","kind":"Deployment","metadata":{"name":"web-app","namespace":"production","labels":{"app":"web","tier":"frontend"},"annotations":{"deployment.kubernetes.io/revision":"3"}},"spec":{"replicas":3,"selector":{"matchLabels":{"app":"web"}},"template":{"metadata":{"labels":{"app":"web"}},"spec":{"containers":[{"name":"web","image":"nginx:1.21","ports":[{"containerPort":80}],"resources":{"limits":{"cpu":"500m","memory":"128Mi"},"requests":{"cpu":"250m","memory":"64Mi"}},"livenessProbe":{"httpGet":{"path":"/healthz","port":80},"initialDelaySeconds":3,"periodSeconds":10},"env":[{"name":"ENV","value":"production"},{"name":"LOG_LEVEL","value":"info"}]}]}}}}'
fi

# ===================
# HASH
# ===================

if [[ "$TYPE" == "all" || "$TYPE" == "hash" ]]; then

    # Single field
    $CLI HSET "hash:single" only "value"

    # Few fields (simple)
    $CLI HSET "hash:few" name "Alice" email "alice@example.com" age "28"

    # Few fields with JSON values
    $CLI HSET "hash:json" \
        config '{"timeout":30,"retries":3}' \
        users '[{"id":1},{"id":2}]' \
        simple "plain text"

    # Many fields (600)
    for i in $(seq 1 600); do
        $CLI HSET "hash:many" "field_$(printf '%03d' "$i")" "value_$i"
    done
fi

# ===================
# LIST
# ===================

if [[ "$TYPE" == "all" || "$TYPE" == "list" ]]; then

    # Single item
    $CLI RPUSH "list:single" "only item"

    # Few items
    $CLI RPUSH "list:few" "first" "second" "third"

    # Many items (600)
    for i in $(seq 1 600); do
        $CLI RPUSH "list:many" "item-$(printf '%03d' "$i")"
    done

    # JSON items
    $CLI RPUSH "list:json" \
        '{"id":1,"type":"task","title":"Review PR"}' \
        '{"id":2,"type":"bug","title":"Fix login"}' \
        '{"id":3,"type":"feature","title":"Add export"}'
fi

# ===================
# SET
# ===================

if [[ "$TYPE" == "all" || "$TYPE" == "set" ]]; then

    # Single member
    $CLI SADD "set:single" "only member"

    # Few members
    $CLI SADD "set:few" "alpha" "beta" "gamma"

    # Many members (600)
    for i in $(seq 1 600); do
        $CLI SADD "set:many" "member-$(printf '%03d' "$i")"
    done

    # JSON members
    $CLI SADD "set:json" \
        '{"id":"a","name":"first"}' \
        '{"id":"b","name":"second"}' \
        '{"id":"c","name":"third"}'
fi

# ===================
# SORTED SET (ZSET)
# ===================

if [[ "$TYPE" == "all" || "$TYPE" == "zset" ]]; then

    # Single member
    $CLI ZADD "zset:single" 100 "only member"

    # Few members
    $CLI ZADD "zset:few" 1 "first" 2 "second" 3 "third"

    # Many members (600)
    for i in $(seq 1 600); do
        $CLI ZADD "zset:many" "$i" "member-$(printf '%03d' "$i")"
    done

    # Negative and zero scores
    $CLI ZADD "zset:negative" -100 "very-low" -50 "low" 0 "zero" 50 "high" 100 "very-high"

    # Same scores (tests ordering)
    $CLI ZADD "zset:same-scores" 100 "alpha" 100 "beta" 100 "gamma" 100 "delta"

    # Float scores
    $CLI ZADD "zset:floats" 1.1 "a" 1.11 "b" 1.111 "c" 2.5 "d" 99.99 "e"

    # JSON members
    $CLI ZADD "zset:json" \
        1705312200 '{"type":"post","content":"Hello world"}' \
        1705315800 '{"type":"comment","content":"Nice!"}' \
        1705319400 '{"type":"share","content":"Check this out"}'
fi

# ===================
# GEO (Geospatial Index)
# ===================

if [[ "$TYPE" == "all" || "$TYPE" == "geo" ]]; then

    # Single location
    $CLI GEOADD "geo:single" -122.4194 37.7749 "San Francisco"

    # Few locations (US cities)
    $CLI GEOADD "geo:cities:us" \
        -122.4194 37.7749 "San Francisco" \
        -118.2437 34.0522 "Los Angeles" \
        -73.9857 40.7484 "New York" \
        -87.6298 41.8781 "Chicago" \
        -95.3698 29.7604 "Houston"

    # International landmarks
    $CLI GEOADD "geo:landmarks" \
        -0.1276 51.5074 "Big Ben" \
        2.2945 48.8584 "Eiffel Tower" \
        12.4924 41.8902 "Colosseum" \
        139.6917 35.6895 "Tokyo Tower" \
        -43.1729 -22.9068 "Christ the Redeemer" \
        151.2153 -33.8568 "Sydney Opera House"

    # Coffee shops (clustered locations for radius search testing)
    $CLI GEOADD "geo:coffee:downtown" \
        -122.4089 37.7851 "Blue Bottle Coffee" \
        -122.4103 37.7879 "Sightglass Coffee" \
        -122.4058 37.7892 "Ritual Coffee Roasters" \
        -122.4127 37.7866 "Philz Coffee" \
        -122.4072 37.7840 "Starbucks Reserve"

    # Many locations (airports)
    $CLI GEOADD "geo:airports" \
        -122.3750 37.6213 "SFO" \
        -118.4085 33.9425 "LAX" \
        -73.7781 40.6413 "JFK" \
        -87.9073 41.9742 "ORD" \
        -95.3414 29.9902 "IAH" \
        -84.4281 33.6407 "ATL" \
        -97.0403 32.8998 "DFW" \
        -104.6737 39.8561 "DEN" \
        -115.1523 36.0840 "LAS" \
        -122.3088 47.4502 "SEA"
fi

# ===================
# STREAM
# ===================

if [[ "$TYPE" == "all" || "$TYPE" == "stream" ]]; then

    # Single entry with few fields
    $CLI XADD "stream:single" "*" event "ping"

    # Few entries
    $CLI XADD "stream:few" "*" action "start" status "ok"
    $CLI XADD "stream:few" "*" action "process" status "ok"
    $CLI XADD "stream:few" "*" action "complete" status "ok"

    # Entry with many fields
    $CLI XADD "stream:wide" "*" \
        field_01 "val01" field_02 "val02" field_03 "val03" field_04 "val04" field_05 "val05" \
        field_06 "val06" field_07 "val07" field_08 "val08" field_09 "val09" field_10 "val10"

    # Many entries (600)
    for i in $(seq 1 600); do
        $CLI XADD "stream:many" "*" n "$i" data "entry-$(printf '%03d' "$i")"
    done

    # JSON field values
    $CLI XADD "stream:json" "*" \
        event "user.created" \
        payload '{"userId":123,"email":"test@example.com"}'
fi

# ===================
# HYPERLOGLOG
# ===================

if [[ "$TYPE" == "all" || "$TYPE" == "hll" ]]; then

    # Single element
    $CLI PFADD "hll:single" "only-element"

    # Few elements
    $CLI PFADD "hll:few" "alpha" "beta" "gamma" "delta" "epsilon"

    # Many elements (demonstrates cardinality estimation)
    for i in $(seq 1 1000); do
        $CLI PFADD "hll:many" "user-$i"
    done

    # Duplicate elements (count should stay same)
    $CLI PFADD "hll:duplicates" "a" "b" "c" "a" "b" "c" "a" "b" "c"

    # High cardinality simulation (unique visitors)
    for i in $(seq 1 10000); do
        $CLI PFADD "hll:visitors" "visitor-$RANDOM-$i"
    done
fi

# ===================
# TTL examples
# ===================

if [[ "$TYPE" == "all" || "$TYPE" == "ttl" ]]; then

    $CLI SET "ttl:short" "expires soon" EX 60
    $CLI SET "ttl:medium" "expires later" EX 3600
    $CLI SET "ttl:long" "expires much later" EX 86400
fi

echo ""
echo "Seeded $(valkey-cli -p "$PORT_VALKEY" DBSIZE | cut -d' ' -f2) keys (type: $TYPE)"
echo "Run 'valkey-cli -p $PORT_VALKEY KEYS \"*\"' to see all keys"
