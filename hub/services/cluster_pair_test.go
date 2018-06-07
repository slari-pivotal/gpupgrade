package services_test

import (
	"errors"
	"fmt"
	"os"

	"github.com/greenplum-db/gpupgrade/hub/services"
	"github.com/greenplum-db/gpupgrade/testutils"
	"github.com/greenplum-db/gpupgrade/utils"

	"github.com/greenplum-db/gp-common-go-libs/testhelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ClusterPair", func() {
	var (
		filesLaidDown []string
		subject       *services.ClusterPair
		testExecutor  *testhelper.TestExecutor
	)

	BeforeEach(func() {
		testhelper.SetupTestLogger()
		testExecutor = &testhelper.TestExecutor{}
		subject = testutils.CreateSampleClusterPair()
		subject.OldBinDir = "old/path"
		subject.NewBinDir = "new/path"
		subject.OldCluster.Executor = testExecutor
	})

	AfterEach(func() {
		utils.System = utils.InitializeSystemFunctions()
		filesLaidDown = []string{}
	})

	Describe("StopEverything(), shutting down both clusters", func() {
		BeforeEach(func() {
			// fake out system utilities
			numInvocations := 0
			utils.System.ReadFile = func(filename string) ([]byte, error) {
				if numInvocations == 0 {
					numInvocations++
					return []byte(testutils.MASTER_ONLY_JSON), nil
				} else {
					return []byte(testutils.NEW_MASTER_JSON), nil
				}
			}
			utils.System.OpenFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
				filesLaidDown = append(filesLaidDown, name)
				return nil, nil
			}
			utils.System.Remove = func(name string) error {
				filteredFiles := make([]string, 0)
				for _, file := range filesLaidDown {
					if file != name {
						filteredFiles = append(filteredFiles, file)
					}
				}
				filesLaidDown = filteredFiles
				return nil
			}
		})

		It("Logs successfully when things work", func() {
			oldRunning, newRunning := subject.EitherPostmasterRunning()
			Expect(oldRunning).To(BeTrue())
			Expect(newRunning).To(BeTrue())

			subject.StopEverything("path/to/gpstop", oldRunning, newRunning)

			Expect(filesLaidDown).To(ContainElement("path/to/gpstop/gpstop.old/completed"))
			Expect(filesLaidDown).To(ContainElement("path/to/gpstop/gpstop.new/completed"))
			Expect(filesLaidDown).ToNot(ContainElement("path/to/gpstop/gpstop.old/running"))
			Expect(filesLaidDown).ToNot(ContainElement("path/to/gpstop/gpstop.new/running"))

			Expect(testExecutor.LocalCommands).To(ContainElement(fmt.Sprintf("source %s/../greenplum_path.sh; %s/gpstop -a -d %s", "old/path", "old/path", "/old/datadir")))
			Expect(testExecutor.LocalCommands).To(ContainElement(fmt.Sprintf("source %s/../greenplum_path.sh; %s/gpstop -a -d %s", "new/path", "new/path", "/new/datadir")))
		})

		It("puts failures in the log if there are filesystem errors", func() {
			utils.System.OpenFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
				return nil, errors.New("filesystem blowup")
			}

			subject.StopEverything("path/to/gpstop", true, true)

			Expect(filesLaidDown).ToNot(ContainElement("path/to/gpstop/gpstop.old/in.progress"))
		})

		It("puts Stop failures in the log and leaves files to mark the error", func() {
			oldRunning, newRunning := subject.EitherPostmasterRunning()
			Expect(oldRunning).To(BeTrue())
			Expect(newRunning).To(BeTrue())

			testExecutor.LocalError = errors.New("generic error")
			subject.StopEverything("path/to/gpstop", oldRunning, newRunning)

			Expect(filesLaidDown).To(ContainElement("path/to/gpstop/gpstop.old/failed"))
			Expect(filesLaidDown).ToNot(ContainElement("path/to/gpstop/gpstop.old/in.progress"))
		})
	})

	Describe("PostmastersRunning", func() {
		BeforeEach(func() {
			utils.System.ReadFile = func(filename string) ([]byte, error) {
				return []byte(testutils.MASTER_ONLY_JSON), nil
			}
			subject.OldCluster.Executor = &testhelper.TestExecutor{}
		})
		It("returns true, true if both postmaster processes are running", func() {
			oldRunning, newRunning := subject.EitherPostmasterRunning()
			Expect(oldRunning).To(BeTrue())
			Expect(newRunning).To(BeTrue())
		})
		It("returns true, false if only old postmaster is running", func() {
			subject.OldCluster.Executor = &testhelper.TestExecutor{
				LocalError:     errors.New("failed"),
				ErrorOnExecNum: 2,
			}
			oldRunning, newRunning := subject.EitherPostmasterRunning()
			Expect(oldRunning).To(BeTrue())
			Expect(newRunning).To(BeFalse())
		})
		It("returns false, true if only new postmaster is running", func() {
			subject.OldCluster.Executor = &testhelper.TestExecutor{
				LocalError:     errors.New("failed"),
				ErrorOnExecNum: 1,
			}
			oldRunning, newRunning := subject.EitherPostmasterRunning()
			Expect(oldRunning).To(BeFalse())
			Expect(newRunning).To(BeTrue())
		})
		It("returns false, false if both postmaster processes are down", func() {
			subject.OldCluster.Executor = &testhelper.TestExecutor{
				LocalError: errors.New("failed"),
			}
			oldRunning, newRunning := subject.EitherPostmasterRunning()
			Expect(oldRunning).To(BeFalse())
			Expect(newRunning).To(BeFalse())
		})
	})
})
