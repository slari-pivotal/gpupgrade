package commanders_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/greenplum-db/gpupgrade/cli/commanders"
	"github.com/greenplum-db/gpupgrade/idl"
	"github.com/greenplum-db/gpupgrade/idl/mock_idl"

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
		name  string
		reply *idl.CheckDiskSpaceReply
		err   error
	}{
		{"reports completion on success",
			&idl.CheckDiskSpaceReply{Failed: map[string]*idl.CheckDiskSpaceReply_DiskUsage{}},
			nil,
		},
		{"reports failure when hub returns full disks",
			&idl.CheckDiskSpaceReply{Failed: map[string]*idl.CheckDiskSpaceReply_DiskUsage{
				"mdw": {Total: 300, Free: 1},
			}},
			errors.New("it failed.."),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			client := mock_idl.NewMockCliToHubClient(ctrl)
			client.EXPECT().CheckDiskSpace(
				gomock.Any(),
				&idl.CheckDiskSpaceRequest{},
			).Return(c.reply, nil)

			d := bufferStandardDescriptors(t)
			defer d.Close()

			err := commanders.CheckDiskSpace(client)

			if err != c.err {
				t.Errorf("returned error %#v, want %#v", err, c.err)
			}

			actualOut, _ := d.Collect()

			expectedStatus := idl.StepStatus_COMPLETE
			if c.err != nil {
				expectedStatus = idl.StepStatus_FAILED
			}
			expected := commanders.Format("Checking disk space...", expectedStatus)

			if !strings.Contains(string(actualOut), expected) {
				t.Errorf("Expected string %q to contain %q", actualOut, expected)
			}
		})
	}
}
