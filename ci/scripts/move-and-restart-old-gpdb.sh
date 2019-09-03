#!/bin/bash
set -euo pipefail

export MASTER_DATA_DIRECTORY=/data/gpdata/master/gpseg-1
export PGPORT=5432

function install_python_hacks_on_host() {
	local node_hostname=$1 gphome_old=$2 gphome_new=$3
	ssh -t centos@"$node_hostname" GPHOME_OLD="${gphome_old}" "sudo bash -c \"
		source /home/gpadmin/common.bash
		install_python_hacks
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_old}/bin/pg_ctl
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_new}/bin/pg_ctl
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_old}/bin/pg_upgrade
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_new}/bin/pg_upgrade
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_old}/bin/pg_dump
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_new}/bin/pg_dump
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_old}/bin/pg_dumpall
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_new}/bin/pg_dumpall
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_old}/bin/pg_restore
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_new}/bin/pg_restore
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_old}/bin/psql
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_new}/bin/psql
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_old}/bin/postgres
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_new}/bin/postgres
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_old}/bin/vacuumdb
		sudo patchelf --set-rpath '\\\$ORIGIN/../lib' ${gphome_new}/bin/vacuumdb
		sudo patchelf --set-rpath '\\\$ORIGIN' ${gphome_old}/lib/libpq.so.5
		sudo patchelf --set-rpath '\\\$ORIGIN' ${gphome_old}/lib/libgssapi_krb5.so.2
		sudo patchelf --set-rpath '\\\$ORIGIN' ${gphome_new}/lib/libpq.so.5
		sudo patchelf --set-rpath '\\\$ORIGIN/../ext/python/lib' ${gphome_old}/lib/postgresql/plpython2.so
	\""
}

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
	scp gpadmin@mdw:/home/gpadmin/gpdb_src/concourse/scripts/common.bash gpadmin@${segment_host}:/home/gpadmin
	install_python_hacks_on_host $segment_host "${GPHOME_OLD}" "${GPHOME_NEW}"
done

start_old_cluster "${GPHOME_OLD}"
