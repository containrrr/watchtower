#!/bin/sh
# Watchtower pre-update lifecycle hook to stop Docker containers on a Synology NAS

set -eu

# ------------------------------------------------------------------
# Configuration (override via environment variables)
# ------------------------------------------------------------------
: "${SYNO_URL:=}" # e.g. https://nas.local:5001
: "${SYNO_USER:=}"
: "${SYNO_PASS:=}"
: "${CLIENT_TIMEOUT:=30}"   # seconds
: "${CLIENT_SSL_VERIFY:=1}" # set to 0 only if you really must disable cert validation

# ------------------------------------------------------------------
# Helper: abort with message
# ------------------------------------------------------------------
abort() {
    printf 'ERROR: %s\n' "$*" >&2
    exit 1
}

# ------------------------------------------------------------------
# Validate configuration
# ------------------------------------------------------------------
[ -n "$SYNO_URL" ] || abort "SYNO_URL environment variable is required"
[ -n "$SYNO_USER" ] || abort "SYNO_USER environment variable is required"
[ -n "$SYNO_PASS" ] || abort "SYNO_PASS environment variable is required"

# ------------------------------------------------------------------
# Validate Watchtower context
# ------------------------------------------------------------------
# This script is only for stopping containers before an update
[ $# -eq 0 ] || abort "Usage: $0 (no arguments expected - stops the container being updated)"

[ -n "${WT_CONTAINER:-}" ] || abort "Watchtower's WT_CONTAINER environment variable is missing"

# ------------------------------------------------------------------
# JSON parser to extract the container name (.name) from WT_CONTAINER
# ------------------------------------------------------------------
parse_container_name() {
    # Input comes from stdin
    if ! awk '
    BEGIN { RS=""; FS="" }
    {
        # Find "name": followed by optional spaces and "
        if (match($0, /"name"[[:space:]]*:[[:space:]]*"(\\.|[^"\\])*"/)) {
            # Extract the content between the quotes
            content = substr($0, RSTART, RLENGTH)
            # Remove "name": " from start and " from end
            sub(/^"name"[[:space:]]*:[[:space:]]*"/, "", content)
            sub(/"$/, "", content)
            # Unescape quotes
            gsub(/\\"/, "\"", content)
            print content
        }
    }' 2>/dev/null; then
        echo "Error: awk failed to parse container name" >&2
        return 1
    fi
}

CONTAINER_NAME=$(printf '%s\n' "$WT_CONTAINER" | parse_container_name) ||
    abort "Failed to parse container name from WT_CONTAINER"

[ -n "$CONTAINER_NAME" ] || abort "Parsed container name is empty"

printf 'Stopping container: %s\n' "$CONTAINER_NAME"

# ------------------------------------------------------------------
# Build curl options
# ------------------------------------------------------------------
CURL_BASE="curl --silent --show-error --max-time $CLIENT_TIMEOUT"
[ "$CLIENT_SSL_VERIFY" = "1" ] || CURL_BASE="$CURL_BASE --insecure"

# ------------------------------------------------------------------
# Authenticate with Synology DSM
# ------------------------------------------------------------------
authenticate() {
    auth_url="${SYNO_URL}/webapi/auth.cgi"
    resp=$($CURL_BASE "$auth_url?api=SYNO.API.Auth&version=6&method=login&account=$SYNO_USER&passwd=$SYNO_PASS&enable_syno_token=yes&format=sid")

    # Extract success
    if ! printf '%s\n' "$resp" | grep -q '"success"[ \t]*:[ \t]*true'; then
        abort "Synology authentication failed (invalid credentials or server error)"
    fi

    # Extract sid and synotoken
    SID=$(printf '%s\n' "$resp" | sed -n 's/.*"sid"[ \t]*:[ \t]*"\([^"]*\)".*/\1/p')
    SYNOTOKEN=$(printf '%s\n' "$resp" | sed -n 's/.*"synotoken"[ \t]*:[ \t]*"\([^"]*\)".*/\1/p')

    [ -n "$SID" ] && [ -n "$SYNOTOKEN" ] || abort "Failed to extract SID or SynoToken"
}

# ------------------------------------------------------------------
# Perform container stop action
# ------------------------------------------------------------------
stop_container() {
    resp=$($CURL_BASE -X POST \
        -H "Content-Type: application/x-www-form-urlencoded; charset=UTF-8" \
        -H "X-SYNO-TOKEN: $SYNOTOKEN" \
        -d "name=$CONTAINER_NAME&api=SYNO.Docker.Container&method=stop&version=1" \
        -b "id=$SID" \
        "${SYNO_URL}/webapi/entry.cgi")

    if printf '%s\n' "$resp" | grep -q '"success":true'; then
        printf 'Container "%s" stopped successfully.\n' "$CONTAINER_NAME"
    else
        printf 'Failed to stop container "%s"\n' "$CONTAINER_NAME" >&2
        printf 'API response: %s\n' "$resp" >&2
        return 1
    fi
}

# ------------------------------------------------------------------
# Logout to clean up session
# ------------------------------------------------------------------
perform_logout() {
    $CURL_BASE "${SYNO_URL}/webapi/auth.cgi?api=SYNO.API.Auth&version=3&method=logout&session=Docker" \
        -b "id=$SID" >/dev/null 2>&1 || true
}

# ------------------------------------------------------------------
# Main execution
# ------------------------------------------------------------------
authenticate
if stop_container; then
    perform_logout
    exit 0
else
    perform_logout
    exit 1
fi
