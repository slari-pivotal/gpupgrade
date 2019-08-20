#!/bin/bash
set -euo pipefail

export MASTER_DATA_DIRECTORY=/data/gpdata/master/gpseg-1
export PGPORT=5432

move_and_update_path() {
	local node_hostname=$1 gphome_old=$2
    ssh -ttn centos@"$node_hostname" GPHOME_OLD="${gphome_old}" '
		sudo mv /usr/local/greenplum-db-devel ${GPHOME_OLD}
		sudo sed -e "s|GPHOME=.*$|GPHOME=$GPHOME_OLD|" -i ${GPHOME_OLD}/greenplum_path.sh
	'
}

stop_old_cluster() {
    # the old cluster is installed to /usr/local/greenplum-db-devel by default
    ssh gpadmin@mdw \
			MASTER_DATA_DIRECTORY=$MASTER_DATA_DIRECTORY \
			PGPORT=$PGPORT bash <<"EOF"
		source /usr/local/greenplum-db-devel/greenplum_path.sh
		gpstop -a
EOF
}

start_old_cluster() {
	local gphome_old=$1
    ssh gpadmin@mdw \
			GPHOME_OLD="${gphome_old}" \
			MASTER_DATA_DIRECTORY=$MASTER_DATA_DIRECTORY \
			PGPORT=$PGPORT bash <<"EOF"
		source ${GPHOME_OLD}/greenplum_path.sh
		gpstart -a
EOF
}

./ccp_src/scripts/setup_ssh_to_cluster.sh

stop_old_cluster

for segment_host in $(cat cluster_env_files/hostfile_all); do
  move_and_update_path $segment_host "${GPHOME_OLD}"
done

start_old_cluster "${GPHOME_OLD}"