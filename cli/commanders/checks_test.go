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
