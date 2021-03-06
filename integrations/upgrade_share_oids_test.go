package integrations_test

import (
	"errors"

	agentServices "github.com/greenplum-db/gpupgrade/agent/services"
	"github.com/greenplum-db/gpupgrade/hub/cluster"
	"github.com/greenplum-db/gpupgrade/hub/configutils"
	hubServices "github.com/greenplum-db/gpupgrade/hub/services"
	"github.com/greenplum-db/gpupgrade/testutils"

	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("upgrade share oids", func() {
	var (
		hub       *hubServices.Hub
		agent     *agentServices.AgentServer
		hubExecer *testutils.FakeCommandExecer
		agentPort int

		outChan            chan []byte
		errChan            chan error
		stubRemoteExecutor *testutils.StubRemoteExecutor
	)

	BeforeEach(func() {
		var err error

		config := `{"SegConfig":[{
			"dbid": 1,
			"port": 5432,
			"host": "localhost"
		}],"BinDir":"/tmp"}`

		testutils.WriteOldConfig(testStateDir, config)
		testutils.WriteNewConfig(testStateDir, config)

		agentPort, err = testutils.GetOpenPort()
		Expect(err).ToNot(HaveOccurred())

		agentConfig := agentServices.AgentConfig{
			Port:     agentPort,
			StateDir: testStateDir,
		}

		agentExecer := &testutils.FakeCommandExecer{}
		agentExecer.SetOutput(&testutils.FakeCommand{})

		agent = agentServices.NewAgentServer(agentExecer.Exec, agentConfig)
		go agent.Start()

		port, err = testutils.GetOpenPort()
		Expect(err).ToNot(HaveOccurred())

		conf := &hubServices.HubConfig{
			CliToHubPort:   port,
			HubToAgentPort: agentPort,
			StateDir:       testStateDir,
		}

		reader := configutils.NewReader()

		outChan = make(chan []byte, 10)
		errChan = make(chan error, 10)
		hubExecer = &testutils.FakeCommandExecer{}
		hubExecer.SetOutput(&testutils.FakeCommand{
			Out: outChan,
			Err: errChan,
		})

		stubRemoteExecutor = testutils.NewStubRemoteExecutor()
		hub = hubServices.NewHub(&cluster.Pair{}, &reader, grpc.DialContext, hubExecer.Exec, conf, stubRemoteExecutor)
		go hub.Start()
	})

	AfterEach(func() {
		hub.Stop()
		agent.Stop()

		Expect(checkPortIsAvailable(port)).To(BeTrue())
		Expect(checkPortIsAvailable(agentPort)).To(BeTrue())
	})

	It("updates status PENDING to RUNNING then to COMPLETE if successful", func() {
		Expect(runStatusUpgrade()).To(ContainSubstring("PENDING - Copy OID files from master to segments"))

		trigger := make(chan struct{}, 1)
		hubExecer.SetTrigger(trigger)

		upgradeShareOidsSession := runCommand("upgrade", "share-oids")
		Eventually(upgradeShareOidsSession).Should(Exit(0))

		Eventually(runStatusUpgrade()).Should(ContainSubstring("RUNNING - Copy OID files from master to segments"))
		trigger <- struct{}{}

		Expect(hubExecer.Calls()[0]).To(ContainSubstring("rsync"))

		Expect(runStatusUpgrade()).To(ContainSubstring("COMPLETE - Copy OID files from master to segments"))
	})

	It("updates status to FAILED if it fails to run", func() {
		Expect(runStatusUpgrade()).To(ContainSubstring("PENDING - Copy OID files from master to segments"))

		errChan <- errors.New("fake test error, share oid failed to send files")

		upgradeShareOidsSession := runCommand("upgrade", "share-oids")
		Eventually(upgradeShareOidsSession).Should(Exit(0))

		Eventually(runStatusUpgrade()).Should(ContainSubstring("FAILED - Copy OID files from master to segments"))
	})
})
