package commanders

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/xerrors"

	"github.com/greenplum-db/gpupgrade/idl"
)

func RunChecks(client idl.CliToHubClient, ratio float32) error {
	err := CheckVersion(client)
	if err != nil {
		return errors.Wrap(err, "checking version compatibility")
	}

	return CheckDiskSpace(client, ratio)
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

type DiskSpaceError struct {
	Failed map[string]*idl.CheckDiskSpaceReply_DiskUsage
}

func (d DiskSpaceError) Error() string {
	var b strings.Builder
	b.WriteString("You currently do not have enough disk space to run an upgrade.\n\n")

	b.WriteString("Expected Space Available:\n")
	for host, disk := range d.Failed {
		b.WriteString(fmt.Sprintf(" - %s: %d\n", host, disk.Total))
	}

	b.WriteString("Actual Space Available:\n")
	for host, disk := range d.Failed {
		b.WriteString(fmt.Sprintf(" - %s: %d\n", host, disk.Free))
	}

	return b.String()
}

func CheckDiskSpace(client idl.CliToHubClient, ratio float32) (err error) {
	s := Substep("Checking disk space...")
	defer s.Finish(&err)

	reply, err := client.CheckDiskSpace(context.Background(), &idl.CheckDiskSpaceRequest{Ratio: ratio})
	if err != nil {
		return xerrors.Errorf("check disk space: %w", err)
	}
	if len(reply.Failed) > 0 {
		return DiskSpaceError{reply.Failed}
	}
	return nil
}
