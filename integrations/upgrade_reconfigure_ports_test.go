package integrations_test

import (
	"errors"
	"io/ioutil"
	"os"
	"time"

	"github.com/greenplum-db/gpupgrade/hub/cluster_ssher"
	"github.com/greenplum-db/gpupgrade/hub/services"
	"github.com/greenplum-db/gpupgrade/hub/upgradestatus"
	"github.com/greenplum-db/gpupgrade/testutils"

	"google.golang.org/grpc"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("upgrade reconfigure ports", func() {

	var (
		dir       string
		hub       *services.Hub
		hubExecer *testutils.FakeCommandExecer
		agentPort int

		outChan chan []byte
		errChan chan error
	)

	BeforeEach(func() {
		var err error
		dir, err = ioutil.TempDir("", "")
		Expect(err).ToNot(HaveOccurred())

		config := `[{
			"dbid": 1,
			"port": 5432,
			"host": "localhost"
		}]`

		testutils.WriteOldConfig(dir, config)
		testutils.WriteNewConfig(dir, config)

		agentPort, err = testutils.GetOpenPort()
		Expect(err).ToNot(HaveOccurred())

		port, err = testutils.GetOpenPort()
		Expect(err).ToNot(HaveOccurred())

		conf := &services.HubConfig{
			CliToHubPort:   port,
			HubToAgentPort: agentPort,
			StateDir:       dir,
		}

		outChan = make(chan []byte, 10)
		errChan = make(chan error, 10)
		hubExecer = &testutils.FakeCommandExecer{}
		hubExecer.SetOutput(&testutils.FakeCommand{
			Out: outChan,
			Err: errChan,
		})

		clusterSsher := cluster_ssher.NewClusterSsher(
			upgradestatus.NewChecklistManager(conf.StateDir),
			services.NewPingerManager(conf.StateDir, 500*time.Millisecond),
			hubExecer.Exec,
		)
		hub = services.NewHub(testutils.InitClusterPairFromDB(), grpc.DialContext, hubExecer.Exec, conf, clusterSsher)
		go hub.Start()
	})

	AfterEach(func() {
		hub.Stop()

		os.RemoveAll(dir)

		Expect(checkPortIsAvailable(port)).To(BeTrue())
		Expect(checkPortIsAvailable(agentPort)).To(BeTrue())
	})

	It("updates status PENDING to COMPLETE if successful", func() {
		Expect(runStatusUpgrade()).To(ContainSubstring("PENDING - Adjust upgrade cluster ports"))

		upgradeReconfigurePortsSession := runCommand("upgrade", "reconfigure-ports")
		Eventually(upgradeReconfigurePortsSession).Should(Exit(0))

		Expect(hubExecer.Calls()[0]).To(ContainSubstring("sed"))

		Expect(runStatusUpgrade()).To(ContainSubstring("COMPLETE - Adjust upgrade cluster ports"))

	})

	It("updates status to FAILED if it fails to run", func() {
		Expect(runStatusUpgrade()).To(ContainSubstring("PENDING - Adjust upgrade cluster ports"))

		errChan <- errors.New("fake test error, reconfigure-ports failed")

		upgradeShareOidsSession := runCommand("upgrade", "reconfigure-ports")
		Eventually(upgradeShareOidsSession).Should(Exit(1))

		Eventually(runStatusUpgrade()).Should(ContainSubstring("FAILED - Adjust upgrade cluster ports"))
	})
})
