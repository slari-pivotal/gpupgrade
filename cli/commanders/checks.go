package commanders

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/greenplum-db/gpupgrade/idl"
)

type DiskSpaceError struct {
	Failed map[string]*idl.CheckDiskSpaceReply_DiskUsage
}

func (dse DiskSpaceError) Error() string {
	return fmt.Sprintf("total %d free %d", dse.Failed["mdw"].Total, dse.Failed["mdw"].Free)
}

func CheckVersion(client idl.CliToHubClient) (err error) {
	s := Substep("Checking version compatibility...")
	defer s.Finish(&err)

	resp, err := client.CheckVersion(context.Background(), &idl.CheckVersionRequest{})
	if err != nil {
		return errors.Wrap(err, "gRPC call to hub failed")
	}
	if !resp.IsVersionCompatible {
		return errors.New("Version Compatibility Check Failed")
	}

	return nil
}

func RunChecks(client idl.CliToHubClient) error {
	err := CheckVersion(client)
	if err != nil {
		return errors.Wrap(err, "checking version compatibility")
	}
	return nil
}

func CheckDiskSpace(client idl.CliToHubClient) (err error) {
	s := Substep("Checking disk space...")
	defer s.Finish(&err)

	reply, _ := client.CheckDiskSpace(context.Background(), &idl.CheckDiskSpaceRequest{})
	if len(reply.Failed) > 0 {
		return DiskSpaceError{reply.Failed}
	}
	return nil
}
