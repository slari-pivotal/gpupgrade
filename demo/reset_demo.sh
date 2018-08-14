#!/bin/bash

ps -ef | grep postgres | grep _upgrade | awk '{print $2}' | xargs kill || true
rm -rf "$MASTER_DATA_DIRECTORY"/../../*_upgrade

rm -rf ~/.gpupgrade
rm -rf ~/gpAdminLogs/

pkill -9 gpupgrade_hub
pkill -9 gpupgrade_agent
