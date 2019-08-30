#!/bin/bash

set -eux -o pipefail

# We'll need this to transfer our built binaries over to the cluster hosts.
./ccp_src/scripts/setup_ssh_to_cluster.sh

# Cache our list of hosts to loop over below.
mapfile -t hosts < cluster_env_files/hostfile_all

# Copy over the SQL dump we pulled from master.
scp sqldump/dump.sql.xz gpadmin@mdw:/tmp/

# Build gpupgrade.
export GOPATH=$PWD/go
export PATH=$GOPATH/bin:$PATH

cd $GOPATH/src/github.com/greenplum-db/gpupgrade
make depend
make

# Install the artifacts onto the cluster machines.
artifacts='gpupgrade gpupgrade_hub gpupgrade_agent'
for host in "${hosts[@]}"; do
    scp $artifacts "gpadmin@$host:${GPHOME_NEW}/bin/"
done
scp ci/scripts/test_gpdb5_database_fixups.sql gpadmin@mdw:/tmp/

# Load the SQL dump into the cluster.
# TODO: do we want to keep this for 5to6?  We want to for 6to6.
echo 'Loading SQL dump...'
time ssh mdw GPHOME_OLD=${GPHOME_OLD} bash <<"EOF"
    set -eux -o pipefail

    source ${GPHOME_OLD}/greenplum_path.sh
    export PGOPTIONS='--client-min-messages=warning'
    unxz < /tmp/dump.sql.xz | psql -f - postgres
EOF

time ssh mdw GPHOME_OLD=${GPHOME_OLD} bash <<"EOF"
    set -eux -o pipefail

    source ${GPHOME_OLD}/greenplum_path.sh
    export PGOPTIONS='--client-min-messages=warning'
    isGPDB5=$(psql -p 5432 -d postgres -A -t -c "select count(version) from version() where version like '%Greenplum Database 5%';")
    if [ $isGPDB5 -eq 1 ]; then
        echo "running test_gpdb5_database_fixups.sql"
        psql -p 5432 -d postgres -U gpadmin -f /tmp/test_gpdb5_database_fixups.sql
    else
        echo "not running test_gpdb5_database_fixups.sql"
    fi
EOF

# Now do the upgrade.
time ssh mdw GPHOME_OLD=${GPHOME_OLD} GPHOME_NEW=${GPHOME_NEW} bash <<"EOF"
    set -eu -o pipefail

    source ${GPHOME_OLD}/greenplum_path.sh
    export PGPORT=5432 # TODO remove the need for this

    wait_for_step() {
        local step="$1"
        local timeout=${2:-60} # default to 60 seconds
        local done=0

        for i in $(seq $timeout); do
            local output=$(gpupgrade status upgrade)
            if [ "$?" -ne "0" ]; then
                echo "$output"
                exit 1
            fi

            local status=$(grep "$step" <<< "$output")

            if [[ $status = *FAILED* ]]; then
                echo "$output"
                exit 1
            fi

            if [[ $status = *"COMPLETE - $step"* ]]; then
                done=1
                echo "$status"
                break
            fi

            sleep 1
        done

        if (( ! $done )); then
            echo "ERROR: timed out waiting for '${step}' to complete"
            exit 1
        fi
    }

    dump_sql_old() {
        local port=$1
        local dumpfile=$2

        echo "Dumping old cluster contents from port ${port} to ${dumpfile}..."

        ssh -n mdw "
            source ${GPHOME_OLD}/greenplum_path.sh
            pg_dumpall -p ${port} -f '$dumpfile'
        "
    }

    dump_sql_new() {
        local port=$1
        local dumpfile=$2

        echo "Dumping new cluster contents from port ${port} to ${dumpfile}..."

        ssh -n mdw "
            source ${GPHOME_NEW}/greenplum_path.sh
            pg_dumpall -p ${port} -f '$dumpfile'
        "
    }

    compare_dumps() {
        local old_dump=$1
        local new_dump=$2

        echo "Comparing dumps at ${old_dump} and ${new_dump}..."

        ssh -n mdw "
            diff -U3 --speed-large-files --ignore-space-change '$old_dump' '$new_dump'
        "
    }

    dump_sql_old 5432 /tmp/old.sql

    source ${GPHOME_NEW}/greenplum_path.sh

    echo "GPHOME_OLD: ${GPHOME_OLD}"
    echo "GPHOME_NEW: ${GPHOME_NEW}"

    gpupgrade prepare init \
              --old-bindir ${GPHOME_OLD}/bin \
              --new-bindir ${GPHOME_NEW}/bin

    gpupgrade prepare start-hub

    gpupgrade check config
    gpupgrade check version
    gpupgrade check seginstall

    gpupgrade prepare start-agents

    gpupgrade prepare init-cluster
    wait_for_step "Initialize new cluster"

    gpupgrade prepare shutdown-clusters
    wait_for_step "Shutdown clusters"

    gpupgrade upgrade convert-master
    wait_for_step "Run pg_upgrade on master" 1200 # twenty minute timeout

    gpupgrade upgrade share-oids
    wait_for_step "Copy OID files from master to segments"

    gpupgrade upgrade convert-primaries
    wait_for_step "Run pg_upgrade on primaries" 1200 # twenty minute timeout

    gpupgrade upgrade validate-start-cluster
    wait_for_step "Validate the upgraded cluster can start up"

    dump_sql_new 5433 /tmp/new.sql
    if ! compare_dumps /tmp/old.sql /tmp/new.sql; then
        echo 'error: before and after dumps differ'
        exit 1
    fi

    echo 'Upgrade successful.'
EOF
