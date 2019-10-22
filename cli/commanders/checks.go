package commanders

import (
	"context"

	"github.com/pkg/errors"

	"github.com/greenplum-db/gpupgrade/idl"
)

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
