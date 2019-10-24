package services

import (
	"context"

	"github.com/greenplum-db/gpupgrade/idl"
	"golang.org/x/sys/unix"
)

func (s *AgentServer) CheckDiskSpace(ctx context.Context, in *idl.CheckDiskSpaceRequest) (*idl.CheckDiskSpaceReply, error) {
	stats := &unix.Statfs_t{}
	err := unix.Statfs(h.source.MasterDataDir(), stats)
	if err != nil {
		return nil, err
	}

	totalSpace := uint64(stats.Bsize) * stats.Blocks
	availableSpace := uint64(stats.Bsize) * stats.Bavail
	requiredSpace := uint64(float64(in.Ratio) * float64(totalSpace))

	reply := &idl.CheckDiskSpaceReply{
		Failed: make(map[string]*idl.CheckDiskSpaceReply_DiskUsage),
	}
	if availableSpace < requiredSpace {
		reply.Failed["localhost"] = &idl.CheckDiskSpaceReply_DiskUsage{
			Required:  requiredSpace,
			Available: availableSpace,
		}
	}

	return reply, nil
}

// func (s *AgentServer) CheckDiskSpaceOnAgents(ctx context.Context, in *idl.CheckDiskSpaceRequestToAgent) (*idl.CheckDiskSpaceReplyFromAgent, error) {
// 	gplog.Info("got a check disk command from the hub")
// 	diskUsage, err := s.GetDiskUsage()
// 	if err != nil {
// 		gplog.Error(err.Error())
// 		return nil, err
// 	}
// 	var listDiskUsages []*idl.FileSysUsage
// 	for k, v := range diskUsage {
// 		listDiskUsages = append(listDiskUsages, &idl.FileSysUsage{Filesystem: k, Usage: v})
// 	}
// 	return &idl.CheckDiskSpaceReplyFromAgent{ListOfFileSysUsage: listDiskUsages}, nil
// }

// // diskUsage() wraps a pair of calls to the gosigar library.
// // This is local repetition of the sys_utils function pointer pattern. If there was more than one of these,
// // we would've refactored.
// // "Adapted" from the gosigar usage example at https://github.com/cloudfoundry/gosigar/blob/master/examples/df.go
// func diskUsage() (map[string]float64, error) {
// 	diskUsagePerFS := make(map[string]float64)
// 	fslist := sigar.FileSystemList{}
// 	err := fslist.Get()
// 	if err != nil {
// 		gplog.Error(err.Error())
// 		return nil, err
// 	}

// 	for _, fs := range fslist.List {
// 		dirName := fs.DirName

// 		usage := sigar.FileSystemUsage{}

// 		err = usage.Get(dirName)
// 		if err != nil {
// 			gplog.Error(err.Error())
// 			return nil, err
// 		}

// 		diskUsagePerFS[dirName] = usage.UsePercent()
// 	}
// 	return diskUsagePerFS, nil
// }
