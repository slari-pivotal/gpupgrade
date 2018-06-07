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

func SaveTargetClusterConfig(clusterPair *ClusterPair, dbConnector *dbconn.DBConn, stateDir string, newBinDir string) (*ClusterPair, error) {
	err := os.MkdirAll(stateDir, 0700)
	if err != nil {
		return clusterPair, err
	}

	segConfigs, err := cluster.GetSegmentConfiguration(dbConnector)
	if err != nil {
		errMsg := fmt.Sprintf("Unable to get segment configuration for new cluster: %s", err.Error())
		return clusterPair, errors.New(errMsg)
	}
	clusterPair.NewCluster = cluster.NewCluster(segConfigs)
	clusterPair.NewBinDir = newBinDir

	err = clusterPair.WriteNewConfig(stateDir)
	return clusterPair, err
}

func (h *Hub) PrepareInitCluster(ctx context.Context, in *pb.PrepareInitClusterRequest) (*pb.PrepareInitClusterReply, error) {
	gplog.Info("starting PrepareInitCluster()")

	dbConnector := db.NewDBConn("localhost", int(in.DbPort), "template1")
	defer dbConnector.Close()
	err := dbConnector.Connect(1)
	if err != nil {
		gplog.Error(err.Error())
		return &pb.PrepareInitClusterReply{}, utils.DatabaseConnectionError{Parent: err}
	}
	dbConnector.Version.Initialize(dbConnector)

	h.clusterPair, err = SaveTargetClusterConfig(h.clusterPair, dbConnector, h.conf.StateDir, in.NewBinDir)
	if err != nil {
		gplog.Error(err.Error())
		return &pb.PrepareInitClusterReply{}, err
	}

	return &pb.PrepareInitClusterReply{}, nil
}
