package services

import (
	"fmt"
	"os"

	"github.com/greenplum-db/gpupgrade/db"
	pb "github.com/greenplum-db/gpupgrade/idl"
	"github.com/greenplum-db/gpupgrade/utils"

	"github.com/greenplum-db/gp-common-go-libs/cluster"
	"github.com/greenplum-db/gp-common-go-libs/dbconn"
	"github.com/greenplum-db/gp-common-go-libs/gplog"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func (h *Hub) CheckConfig(ctx context.Context, in *pb.CheckConfigRequest) (*pb.CheckConfigReply, error) {
	gplog.Info("starting CheckConfig()")

	dbConnector := db.NewDBConn("localhost", int(in.DbPort), "template1")
	defer dbConnector.Close()
	err := dbConnector.Connect(1)
	if err != nil {
		gplog.Error(err.Error())
		return &pb.CheckConfigReply{}, utils.DatabaseConnectionError{Parent: err}
	}
	dbConnector.Version.Initialize(dbConnector)

	h.clusterPair, err = SaveOldClusterConfig(h.clusterPair, dbConnector, h.conf.StateDir, in.OldBinDir)
	if err != nil {
		gplog.Error(err.Error())
		return &pb.CheckConfigReply{}, err
	}

	successReply := &pb.CheckConfigReply{ConfigStatus: "All good"}

	return successReply, nil
}

func SaveOldClusterConfig(clusterPair *ClusterPair, dbConnector *dbconn.DBConn, stateDir string, oldBinDir string) (*ClusterPair, error) {
	err := os.MkdirAll(stateDir, 0700)
	if err != nil {
		return clusterPair, err
	}

	segConfigs, err := cluster.GetSegmentConfiguration(dbConnector)
	if err != nil {
		errMsg := fmt.Sprintf("Unable to get segment configuration for old cluster: %s", err.Error())
		return clusterPair, errors.New(errMsg)
	}
	clusterPair.OldCluster = cluster.NewCluster(segConfigs)
	clusterPair.OldBinDir = oldBinDir

	err = clusterPair.WriteOldConfig(stateDir)
	return clusterPair, err
}
