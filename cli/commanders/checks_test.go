package commanders_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/greenplum-db/gpupgrade/cli/commanders"
	"github.com/greenplum-db/gpupgrade/idl"
	"github.com/greenplum-db/gpupgrade/idl/mock_idl"
	"golang.org/x/xerrors"

	"github.com/golang/mock/gomock"
)

func TestCheckVersion(t *testing.T) {
	t.Run("it prints out version check is OK and that check version request was processed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client := mock_idl.NewMockCliToHubClient(ctrl)
		client.EXPECT().CheckVersion(
			gomock.Any(),
			&idl.CheckVersionRequest{},
		).Return(&idl.CheckVersionReply{IsVersionCompatible: true}, nil)
		err := commanders.CheckVersion(client)

		if err != nil {
			t.Errorf("No error was expected, but got %#v", err)
		}
	})

	t.Run("it prints out version check failed and that check version request was processed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client := mock_idl.NewMockCliToHubClient(ctrl)
		client.EXPECT().CheckVersion(
			gomock.Any(),
			&idl.CheckVersionRequest{},
		).Return(&idl.CheckVersionReply{IsVersionCompatible: false}, nil)
		err := commanders.CheckVersion(client)

		if err == nil {
			t.Fatalf("An error was expected, but got nil")
		}
		expectedError := "Version Compatibility Check Failed"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error to contain %q but got: %#v", expectedError, err)
		}
	})

	t.Run("it prints out that it was unable to connect to hub", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		client := mock_idl.NewMockCliToHubClient(ctrl)
		client.EXPECT().CheckVersion(
			gomock.Any(),
			&idl.CheckVersionRequest{},
		).Return(&idl.CheckVersionReply{IsVersionCompatible: false}, errors.New("something went wrong"))
		err := commanders.CheckVersion(client)

		if err == nil {
			t.Fatalf("An error was expected, but got nil")
		}
		expectedError := "gRPC call to hub failed"
		if !strings.Contains(err.Error(), expectedError) {
			t.Errorf("Expected error to contain %q but got: %#v", expectedError, err)
		}
	})
}

func TestDiskSpaceCheck(t *testing.T) {
	cases := []struct {
		name    string
		failed  map[string]*idl.CheckDiskSpaceReply_DiskUsage
		grpcErr error
	}{
		{"reports completion on success",
			map[string]*idl.CheckDiskSpaceReply_DiskUsage{},
			nil,
		},
		{"reports failure when hub returns full disks",
			map[string]*idl.CheckDiskSpaceReply_DiskUsage{
				"mdw": {Required: 300, Available: 1},
			},
			nil,
		},
		{"reports failure on gRPC error",
			map[string]*idl.CheckDiskSpaceReply_DiskUsage{},
			errors.New("gRPC failure"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// exact value doesn't matter; we simply verify that it's passed
			// through to gRPC as-is
			ratio := float32(0.5)

			client := mock_idl.NewMockCliToHubClient(ctrl)
			client.EXPECT().CheckDiskSpace(
				gomock.Any(),
				&idl.CheckDiskSpaceRequest{Ratio: ratio},
			).Return(&idl.CheckDiskSpaceReply{Failed: c.failed}, c.grpcErr)

			d := bufferStandardDescriptors(t)
			defer d.Close()

			err := commanders.CheckDiskSpace(client, ratio)
			actualOut, _ := d.Collect()

			expectedStatus := idl.StepStatus_FAILED
			switch {
			case c.grpcErr != nil:
				if !xerrors.Is(err, c.grpcErr) {
					t.Errorf("returned error %#v, want %#v", err, c.grpcErr)
				}

			case len(c.failed) != 0:
				var diskSpaceError commanders.DiskSpaceError
				if !xerrors.As(err, &diskSpaceError) {
					t.Errorf("returned error %#v, want a DiskSpaceError", err)
				} else if !reflect.DeepEqual(diskSpaceError.Failed, c.failed) {
					t.Errorf("error contents were %v, want %v", diskSpaceError.Failed, c.failed)
				}

			default:
				expectedStatus = idl.StepStatus_COMPLETE
				if err != nil {
					t.Errorf("returned error %#v, expected no error", err)
				}
			}

			expected := commanders.Format("Checking disk space...", expectedStatus)
			if !strings.Contains(string(actualOut), expected) {
				t.Errorf("Expected string %q to contain %q", actualOut, expected)
			}
		})
	}
}
