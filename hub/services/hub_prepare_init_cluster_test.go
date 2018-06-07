package services_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/greenplum-db/gpupgrade/hub/services"
	"github.com/greenplum-db/gpupgrade/utils"

	"github.com/greenplum-db/gp-common-go-libs/dbconn"
	"github.com/greenplum-db/gp-common-go-libs/testhelper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
)

var _ = Describe("Hub prepare init-cluster", func() {
	var (
		dbConnector *dbconn.DBConn
		mock        sqlmock.Sqlmock
		dir         string
		err         error
		newBinDir   string
		queryResult = `{"SegConfigs":[{"DbID":1,"ContentID":-1,"Port":15432,"Hostname":"mdw","DataDir":"/data/master/gpseg-1"},` +
			`{"DbID":2,"ContentID":0,"Port":25432,"Hostname":"sdw1","DataDir":"/data/primary/gpseg0"}],"BinDir":"/tmp"}`
		clusterPair *services.ClusterPair
	)

	BeforeEach(func() {
		newBinDir = "/tmp"
		dbConnector, mock = testhelper.CreateAndConnectMockDB(1)
		dir, err = ioutil.TempDir("", "")
		Expect(err).ToNot(HaveOccurred())
		utils.System = utils.InitializeSystemFunctions()
		clusterPair = &services.ClusterPair{}
	})

	It("successfully stores target cluster config for GPDB 6", func() {
		testhelper.SetDBVersion(dbConnector, "6.0.0")

		mock.ExpectQuery("SELECT .*").WillReturnRows(getFakeConfigRows())

		fakeConfigFile := gbytes.NewBuffer()
		utils.System.WriteFile = func(filename string, data []byte, perm os.FileMode) error {
			fmt.Fprint(fakeConfigFile, string(data))
			return nil
		}

		newClusterPair, err := services.SaveTargetClusterConfig(clusterPair, dbConnector, dir, newBinDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(fakeConfigFile.Contents())).To(ContainSubstring(queryResult))
		Expect(newClusterPair).To(Equal(clusterPair))
	})

	It("successfully stores target cluster config for GPDB 4 and 5", func() {
		mock.ExpectQuery("SELECT .*").WillReturnRows(getFakeConfigRows())

		fakeConfigFile := gbytes.NewBuffer()
		utils.System.WriteFile = func(filename string, data []byte, perm os.FileMode) error {
			fmt.Fprint(fakeConfigFile, string(data))
			return nil
		}

		newClusterPair, err := services.SaveTargetClusterConfig(clusterPair, dbConnector, dir, newBinDir)
		Expect(err).ToNot(HaveOccurred())

		Expect(string(fakeConfigFile.Contents())).To(ContainSubstring(queryResult))
		Expect(newClusterPair).To(Equal(clusterPair))
	})

	It("fails to get config file handle", func() {
		utils.System.WriteFile = func(filename string, data []byte, perm os.FileMode) error {
			return errors.New("failed to write config file")
		}

		_, err := services.SaveTargetClusterConfig(clusterPair, dbConnector, dir, newBinDir)
		Expect(err).To(HaveOccurred())
	})

	It("db.Select query for cluster config fails", func() {
		mock.ExpectQuery("SELECT .*").WillReturnError(errors.New("fail config query"))

		utils.System.WriteFile = func(filename string, data []byte, perm os.FileMode) error {
			return nil
		}

		_, err := services.SaveTargetClusterConfig(clusterPair, dbConnector, dir, newBinDir)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError("Unable to get segment configuration for new cluster: fail config query"))
	})
})
