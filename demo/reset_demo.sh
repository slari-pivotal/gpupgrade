#!/bin/bash

ps -ef | grep postgres | grep _upgrade | awk '{print $2}' | xargs kill || true
rm -rf "$MASTER_DATA_DIRECTORY"/../../*_upgrade

rm -rf ~/.gpupgrade
rm -rf ~/gpAdminLogs/

pkill -9 gpupgrade_hub
pkill -9 gpupgrade_agent

source /usr/local/4/greenplum_path.sh
source ~/workspace/gpdb4/gpAux/gpdemo/gpdemo-env.sh

export SOURCE_GPHOME=/usr/local/4
export TARGET_GPHOME=/usr/local/5

psql -d postgres -f ./demo/load.sql
