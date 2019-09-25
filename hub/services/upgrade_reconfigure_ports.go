package services

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"

	"golang.org/x/xerrors"

	"github.com/greenplum-db/gpupgrade/hub/upgradestatus"
	"github.com/greenplum-db/gpupgrade/idl"

	"github.com/greenplum-db/gp-common-go-libs/gplog"
)

func (h *Hub) UpgradeReconfigurePorts(ctx context.Context, in *idl.UpgradeReconfigurePortsRequest) (*idl.UpgradeReconfigurePortsReply, error) {
	gplog.Info("starting %s", upgradestatus.RECONFIGURE_PORTS)

	step, err := h.InitializeStep(upgradestatus.RECONFIGURE_PORTS)
	if err != nil {
		gplog.Error(err.Error())
		return &idl.UpgradeReconfigurePortsReply{}, err
	}

	if err := h.reconfigurePorts(); err != nil {
		gplog.Error(err.Error())

		// Log any stderr from failed commands.
		var exitErr *exec.ExitError
		if xerrors.As(err, &exitErr) {
			gplog.Debug(string(exitErr.Stderr))
		}

		step.MarkFailed()
		return &idl.UpgradeReconfigurePortsReply{}, err
	}

	step.MarkComplete()
	return &idl.UpgradeReconfigurePortsReply{}, nil
}

// reconfigurePorts executes the tricky sequence of operations required to
// change the ports on a cluster:
//    1). bring down the cluster
//    2). bring up the master(fts will not "freak out", etc)
//    3). rewrite gp_segment_configuration with the updated port number
//    4). bring down the master
//    5). modify the master's config file to use the new port
//    6). bring up the cluster
// TODO: this method needs test coverage.
func (h *Hub) reconfigurePorts() (err error) {
	// 1). bring down the cluster
	err = StopCluster(h.target)
	if err != nil {
		return xerrors.Errorf("%s failed to stop cluster: %w",
			upgradestatus.RECONFIGURE_PORTS, err)
	}

	// 2). bring up the master(fts will not "freak out", etc)
	script := fmt.Sprintf("source %s/../greenplum_path.sh && %s/gpstart -am -d %s",
		h.target.BinDir, h.target.BinDir, h.target.MasterDataDir())
	cmd := exec.Command("bash", "-c", script)
	_, err = cmd.Output()
	if err != nil {
		return xerrors.Errorf("%s failed to start target cluster in utility mode: %w",
			upgradestatus.RECONFIGURE_PORTS, err)
	}

	// 3). rewrite gp_segment_configuration with the updated port number
	connURI := fmt.Sprintf("postgresql://localhost:%d/template1?gp_session_role=utility&allow_system_table_mods=true&search_path=", h.target.MasterPort())
	targetDB, err := sql.Open("pgx", connURI)
	defer func() {
		targetDB.Close() //TODO: return multierror here to capture err from Close()
	}()
	if err != nil {
		return xerrors.Errorf("%s failed to open connection to utility master: %w",
			upgradestatus.RECONFIGURE_PORTS, err)
	}
	err = ClonePortsFromCluster(targetDB, h.source.Cluster)
	if err != nil {
		return xerrors.Errorf("%s failed to clone ports: %w",
			upgradestatus.RECONFIGURE_PORTS, err)
	}

	// 4). bring down the master
	script = fmt.Sprintf("source %s/../greenplum_path.sh && %s/gpstop -aim -d %s",
		h.target.BinDir, h.target.BinDir, h.target.MasterDataDir())
	cmd = exec.Command("bash", "-c", script)
	_, err = cmd.Output()
	if err != nil {
		return xerrors.Errorf("%s failed to stop target cluster in utility mode: %w",
			upgradestatus.RECONFIGURE_PORTS, err)
	}

	// 5). rewrite the "port" field in the master's postgresql.conf
	script = fmt.Sprintf(
		"sed 's/port=%d/port=%d/' %[3]s/postgresql.conf > %[3]s/postgresql.conf.updated && "+
			"mv %[3]s/postgresql.conf %[3]s/postgresql.conf.bak && "+
			"mv %[3]s/postgresql.conf.updated %[3]s/postgresql.conf",
		h.target.MasterPort(), h.source.MasterPort(), h.target.MasterDataDir(),
	)
	gplog.Debug("executing command: %+v", script) // TODO: Move this debug log into ExecuteLocalCommand()
	cmd = exec.Command("bash", "-c", script)
	_, err = cmd.Output()
	if err != nil {
		return xerrors.Errorf("%s failed to execute sed command: %w",
			upgradestatus.RECONFIGURE_PORTS, err)
	}

	// 6. bring up the cluster
	script = fmt.Sprintf("source %s/../greenplum_path.sh && %s/gpstart -a -d %s",
		h.target.BinDir, h.target.BinDir, h.target.MasterDataDir())
	cmd = exec.Command("bash", "-c", script)
	_, err = cmd.Output()
	if err != nil {
		return xerrors.Errorf("%s failed to start target cluster: %w",
			upgradestatus.RECONFIGURE_PORTS, err)
	}

	return nil
}
