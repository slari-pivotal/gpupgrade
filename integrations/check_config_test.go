package integrations_test

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/greenplum-db/gpupgrade/hub/services"
	"github.com/greenplum-db/gpupgrade/testutils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"google.golang.org/grpc"
)

// needs the cli and the hub
var _ = Describe("check config", func() {
	var (
		dir            string
		hub            *services.Hub
		commandExecer  *testutils.FakeCommandExecer
		hubToAgentPort int
	)

	BeforeEach(func() {
		hubToAgentPort = 6416

		var err error
		dir, err = ioutil.TempDir("", "")
		Expect(err).ToNot(HaveOccurred())

		// We only needed to get the name of the temp directory, so we delete it.
		// The actual directory will be created by the
		// SaveOldClusterConfigAndVersion() routine.
		// Being a temp dir, Go will remove the directory at the end of test also.
		err = os.RemoveAll(dir)
		Expect(err).ToNot(HaveOccurred())

		port, err = testutils.GetOpenPort()
		Expect(err).ToNot(HaveOccurred())

		conf := &services.HubConfig{
			CliToHubPort:   port,
			HubToAgentPort: hubToAgentPort,
			StateDir:       dir,
		}
		commandExecer = &testutils.FakeCommandExecer{}
		commandExecer.SetOutput(&testutils.FakeCommand{})

		hub = services.NewHub(testutils.InitClusterPairFromDB(), grpc.DialContext, commandExecer.Exec, conf, nil)
		go hub.Start()
	})

	AfterEach(func() {
		hub.Stop()
		os.RemoveAll(dir)
	})

	It("happy: the database configuration is saved to a specified location", func() {
		session := runCommand("check", "config", "--master-host", "localhost", "--old-bindir", "/non/existent/path")
		if session.ExitCode() != 0 {
			fmt.Println("make sure greenplum is running")
		}
		Expect(session).To(Exit(0))

		cp := &services.ClusterPair{}
		err := cp.ReadOldConfig(dir)
		testutils.Check("cannot read config", err)

		Expect(len(cp.OldCluster.Segments)).To(BeNumerically(">", 1))
	})

	It("fails if required flags are missing", func() {
		checkConfigSession := runCommand("check", "config")
		Expect(checkConfigSession).Should(Exit(1))
		Expect(string(checkConfigSession.Out.Contents())).To(Equal("Required flag(s) \"master-host\", \"old-bindir\" have/has not been set\n"))
	})
})
