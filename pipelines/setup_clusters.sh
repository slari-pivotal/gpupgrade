#!/bin/bash
#
# Derived from the test_upgrade_sh script under https://github.com/greenplum-db/gp-performance-testing/tree/dev/pg_upgrade
#
set -euo pipefail

# Set the DEBUG_UPGRADE envvar to a nonempty value to get (extremely) verbose
# output.
DEBUG_UPGRADE=${DEBUG_UPGRADE:-}

./ccp_src/scripts/setup_ssh_to_cluster.sh

DIRNAME=$(dirname "$0")

cat << EOF
  ############################
  #                          #
  #  New GPDB Installation   #
  #                          #
  ############################
EOF

load_old_db_data() {
    # Copy the SQL dump over to the master host and load it into the database.
    local dumpfile=$1
    local psql_env="PGOPTIONS='--client-min-messages=warning'"
    local psql_opts="-q"

    if [ -n "$DEBUG_UPGRADE" ]; then
        # Don't quiet psql when debugging.
        psql_env=
        psql_opts=
    fi

    echo 'Loading test database...'

    scp "$dumpfile" mdw:/tmp/dump.sql.xz
    ssh -n mdw '
        source /usr/local/greenplum-db-devel/greenplum_path.sh
        unxz < /tmp/dump.sql.xz | '"${psql_env}"' psql '"${psql_opts}"' -f - postgres
    '
}

dump_cluster() {
    # Dump the entire cluster contents to file, using the new pg_dumpall.
    local dumpfile=$1

    ssh -n mdw "
        source /usr/local/gpdb_master/greenplum_path.sh
        pg_dumpall -f '$dumpfile'
    "
}

extract_gpdb_tarball() {
    local node_hostname=$1
    local tarball_dir=$2
    # Commonly the incoming binary will be called bin_gpdb.tar.gz. Because many other teams/pipelines tend to use 
    # that naming convention we are not, deliberately. Once the file crosses into our domain, we will not use
    # the conventional name.  This should make clear that we will install any valid binary, not just those that follow
    # the naming convention.
    scp ${tarball_dir}/*.tar.gz $node_hostname:/tmp/gpdb_binary.tar.gz
    ssh -ttn $node_hostname "sudo bash -c \"\
      mkdir -p /usr/local/gpdb_master; \
      tar -xf /tmp/gpdb_binary.tar.gz -C /usr/local/gpdb_master; \
      chown -R gpadmin:gpadmin /usr/local/gpdb_master; \
      sed -ie 's|^GPHOME=.*|GPHOME=/usr/local/gpdb_master|' /usr/local/gpdb_master/greenplum_path.sh ; \
    \""
}

create_new_datadir() {
    local node_hostname=$1

    # Create a -new directory for every data directory that already exists.
    # This is what we'll be init'ing the new database into.
    ssh -ttn "$node_hostname" 'sudo bash -c '\''
        for dir in $(find /data/gpdata/* -maxdepth 0 -type d); do
            newdir="${dir}-new"

            mkdir -p "$newdir"
            chown gpadmin:gpadmin "$newdir"
        done
    '\'''
}

gpinitsystem_for_upgrade() {
    # Stop the old cluster and init a new one.
    ssh -ttn mdw '
        source /usr/local/greenplum-db-devel/greenplum_path.sh
        gpstop -a -d /data/gpdata/master/gpseg-1

        source /usr/local/gpdb_master/greenplum_path.sh
        sed -e '\''s|\(/data/gpdata/\w\+\)|\1-new|g'\'' gpinitsystem_config > gpinitsystem_config_new
        echo "HEAP_CHECKSUM=off" >> gpinitsystem_config_new
        gpinitsystem -a -c ~gpadmin/gpinitsystem_config_new -h ~gpadmin/segment_host_list
        gpstop -a -d /data/gpdata/master-new/gpseg-1
    '
}

# run_upgrade hostname data-directory [options]
run_upgrade() {
    # Runs pg_upgrade on a host for the given data directory. The new data
    # directory is assumed to follow the *-new convention established by
    # gpinitsystem_for_upgrade(), above.

    local hostname=$1
    local datadir=$2
    shift 2

    local upgrade_opts=

    if [ -n "$DEBUG_UPGRADE" ]; then
        upgrade_opts="--verbose"
    fi

    ssh -ttn "$hostname" '
        source /usr/local/gpdb_master/greenplum_path.sh
        time pg_upgrade '"${upgrade_opts}"' '"$*"' \
            -b /usr/local/greenplum-db-devel/bin/ -B /usr/local/gpdb_master/bin/ \
            -d '"$datadir"' \
            -D '"$(sed -e 's|\(/data/gpdata/\w\+\)|\1-new|g' <<< "$datadir")"
}

dump_old_master_query() {
    # Prints the rows generated by the given SQL query to stdout. The query is
    # run on the old master, pre-upgrade.
    ssh -n mdw '
        source /usr/local/greenplum-db-devel/greenplum_path.sh
        psql postgres --quiet --no-align --tuples-only -F"'$'\t''" -c "'$1'"
    '
}

get_segment_datadirs() {
    # Prints the hostnames and data directories of each primary and mirror: one
    # instance per line, with the hostname and data directory separated by a
    # tab.

    # First try dumping the 6.0 version...
    local q="SELECT hostname, datadir FROM gp_segment_configuration WHERE content <> -1"
    if ! dump_old_master_query "$q" 2>/dev/null; then
        # ...and then fall back to pre-6.0.
        q="SELECT hostname, fselocation FROM gp_segment_configuration JOIN pg_catalog.pg_filespace_entry ON (dbid = fsedbid) WHERE content <> -1"
        dump_old_master_query "$q"
    fi
}

generate_environment_scripts() {
	ssh -ttn mdw "cat > ~gpadmin/src_old_env.sh <<HERE
export PATH=/usr/local/bin:/bin:/usr/bin:/usr/local/sbin:/usr/sbin:/sbin:/home/gpadmin/bin
export MASTER_DATA_DIRECTORY=/data/gpdata/master/gpseg-1
export PGPORT=5432
source /usr/local/greenplum-db-devel/greenplum_path.sh
alias \"newenv\"=\"source ~gpadmin/src_new_env.sh\"
HERE
cat > ~gpadmin/src_new_env.sh <<HERE
export PATH=/usr/local/bin:/bin:/usr/bin:/usr/local/sbin:/usr/sbin:/sbin:/home/gpadmin/bin
export MASTER_DATA_DIRECTORY=/data/gpdata/master-new/gpseg-1
export PGPORT=6432
source /usr/local/gpdb_master/greenplum_path.sh
alias \"oldenv\"=\"source ~gpadmin/src_old_env.sh\"
HERE"
}

CLUSTER_NAME=$(cat ./terraform*/name)

NUMBER_OF_NODES=$1

if [ -z ${NUMBER_OF_NODES} ]; then
  echo "Number of nodes must be supplied to this script"
  exit 1
fi

GPDB_TARBALL_DIR=$2

if [ -z "{GPDB_TARBALL_DIR}" ]; then
  echo "Using default directory"
fi

set -v

for ((i=0; i<${NUMBER_OF_NODES}; ++i)); do
  extract_gpdb_tarball ccp-${CLUSTER_NAME}-$i ${GPDB_TARBALL_DIR:-gpdb_binary}
  create_new_datadir ccp-${CLUSTER_NAME}-$i
done

get_segment_datadirs > /tmp/segment_datadirs.txt
gpinitsystem_for_upgrade

generate_environment_scripts

echo "Finished with setup"

