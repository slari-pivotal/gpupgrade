package services

import (
	"context"
	"sync"

	"golang.org/x/xerrors"

	"github.com/greenplum-db/gpupgrade/idl"
	multierror "github.com/hashicorp/go-multierror"
)

func (h *Hub) CheckDiskSpace(ctx context.Context, in *idl.CheckDiskSpaceRequest) (*idl.CheckDiskSpaceReply, error) {
	reply := &idl.CheckDiskSpaceReply{
		Failed: make(map[string]*idl.CheckDiskSpaceReply_DiskUsage),
	}

	agents, err := h.AgentConns()
	if err != nil {
		return reply, err
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(agents))
	failures := make(chan map[string]*idl.CheckDiskSpaceReply_DiskUsage, len(agents))

	for i := range agents {
		agent := agents[i]
		wg.Add(1)

		go func() {
			defer wg.Done()

			reply, err := agent.AgentClient.CheckDiskSpace(ctx, in)
			if err != nil {
				errs <- xerrors.Errorf("check disk space on host %s: %w", agent.Hostname, err)
				return
			}

			if len(reply.Failed) > 0 {
				failures <- reply.Failed
			}
		}()
	}

	wg.Wait()
	close(errs)
	close(failures)

	var multiErr *multierror.Error
	for err := range errs {
		multiErr = multierror.Append(multiErr, err)
	}
	if err := multiErr.ErrorOrNil(); err != nil {
		return reply, err
	}

	for failure := range failures {
		for k, v := range failure {
			reply[k] = v
		}
	}
	return reply, nil

	// stats := &unix.Statfs_t{}
	// err := unix.Statfs(h.source.MasterDataDir(), stats)
	// if err != nil {
	// 	return nil, err
	// }

	// totalSpace := uint64(stats.Bsize) * stats.Blocks
	// availableSpace := uint64(stats.Bsize) * stats.Bavail
	// requiredSpace := uint64(float64(in.Ratio) * float64(totalSpace))

	// reply := &idl.CheckDiskSpaceReply{
	// 	Failed: make(map[string]*idl.CheckDiskSpaceReply_DiskUsage),
	// }
	// if availableSpace < requiredSpace {
	// 	reply.Failed["localhost"] = &idl.CheckDiskSpaceReply_DiskUsage{
	// 		Required:  requiredSpace,
	// 		Available: availableSpace,
	// 	}
	// }

	// return reply, nil
}
