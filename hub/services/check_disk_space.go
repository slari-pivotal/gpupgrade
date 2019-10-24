package services

import (
	"context"

	"golang.org/x/sys/unix"

	"github.com/greenplum-db/gpupgrade/idl"
)

func (h *Hub) CheckDiskSpace(ctx context.Context, in *idl.CheckDiskSpaceRequest) (*idl.CheckDiskSpaceReply, error) {
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
			Total: requiredSpace,
			Free:  availableSpace,
		}
	}

	return reply, nil
}
