#! /usr/bin/env bats

load helpers

setup() {
    skip_if_no_gpdb

    STATE_DIR=`mktemp -d`
    export GPUPGRADE_HOME="${STATE_DIR}/gpupgrade"

    gpupgrade kill-services
}

teardown() {
    skip_if_no_gpdb

    gpupgrade kill-services
    rm -r "$STATE_DIR"
}

# gnu_stat tries to find a GNU stat program and prints its path to stdout. If
# none is found, no output is printed.
gnu_stat() {
    path=$(which gstat)
    if [ -z "$path" ]; then
        path=$(which stat)
        if [ -z "$path" ]; then
            return 0
        fi
    fi

    # Check to make sure what we have is really GNU.
    version=$("$path" --version)
    [[ $version = *"GNU coreutils"* ]] && echo $path
    return 0
}

@test "initialize prints disk space on failure" {
    stat=$(gnu_stat)
    [ -n "$stat" ] || skip "GNU stat is required for this test"

    datadir=$(psql postgres -Atc "select datadir from gp_segment_configuration where role='p' and content=-1")
    space_before=$("$stat" -f -c '%a*%S' "$datadir" | bc)

    run gpupgrade initialize \
        --disk-free-ratio=1.0 \
        --old-bindir="$PWD" \
        --new-bindir="$PWD" \
        --old-port="${PGPORT}" 3>&-

    [ "$status" -eq 1 ]

    space_after=$("$stat" -f -c '%a*%S' "$datadir" | bc)

    pattern='You currently do not have enough disk space to run an upgrade\..+'
    pattern+='Expected Space Available:.+'
    pattern+='localhost: ([[:digit:]]+).+'
    pattern+='Actual Space Available:.+'
    pattern+='localhost: ([[:digit:]]+)'

    [[ $output =~ $pattern ]] || fail "actual output: $output"

    expected_space="${BASH_REMATCH[1]}"
    total_space=$("$stat" -f -c '%b*%S' "$datadir" | bc)
    [ "$expected_space" = "$total_space" ] \
        || fail "wanted $expected_space to be $total_space; actual output: $output"

    if (( $space_before < $space_after )); then
        min="$space_before"
        max="$space_after"
    else
        min="$space_after"
        max="$space_before"
    fi
    allowable_threshold=$((10 * 1024 * 1024))
    (( min -= $allowable_threshold ))
    (( max += $allowable_threshold ))

    actual_space="${BASH_REMATCH[2]}"
    (( $min <= $actual_space && $actual_space <= $max )) \
        || fail "wanted $actual_space to be between $min and $max; actual output: $output"
}
