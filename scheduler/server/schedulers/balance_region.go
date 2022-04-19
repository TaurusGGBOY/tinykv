// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package schedulers

import (
	"fmt"
	"github.com/pingcap-incubator/tinykv/scheduler/server/core"
	"github.com/pingcap-incubator/tinykv/scheduler/server/schedule"
	"github.com/pingcap-incubator/tinykv/scheduler/server/schedule/operator"
	"github.com/pingcap-incubator/tinykv/scheduler/server/schedule/opt"
	"sort"
)

func init() {
	schedule.RegisterSliceDecoderBuilder("balance-region", func(args []string) schedule.ConfigDecoder {
		return func(v interface{}) error {
			return nil
		}
	})
	schedule.RegisterScheduler("balance-region", func(opController *schedule.OperatorController, storage *core.Storage, decoder schedule.ConfigDecoder) (schedule.Scheduler, error) {
		return newBalanceRegionScheduler(opController), nil
	})
}

const (
	// balanceRegionRetryLimit is the limit to retry schedule for selected store.
	balanceRegionRetryLimit = 10
	balanceRegionName       = "balance-region-scheduler"
)

type balanceRegionScheduler struct {
	*baseScheduler
	name         string
	opController *schedule.OperatorController
}

// newBalanceRegionScheduler creates a scheduler that tends to keep regions on
// each store balanced.
func newBalanceRegionScheduler(opController *schedule.OperatorController, opts ...BalanceRegionCreateOption) schedule.Scheduler {
	base := newBaseScheduler(opController)
	s := &balanceRegionScheduler{
		baseScheduler: base,
		opController:  opController,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// BalanceRegionCreateOption is used to create a scheduler with an option.
type BalanceRegionCreateOption func(s *balanceRegionScheduler)

func (s *balanceRegionScheduler) GetName() string {
	if s.name != "" {
		return s.name
	}
	return balanceRegionName
}

func (s *balanceRegionScheduler) GetType() string {
	return "balance-region"
}

func (s *balanceRegionScheduler) IsScheduleAllowed(cluster opt.Cluster) bool {
	return s.opController.OperatorCount(operator.OpRegion) < cluster.GetRegionScheduleLimit()
}

type storeInfo []*core.StoreInfo

func (s storeInfo) Len() int { return len(s) }

func (s storeInfo) Less(i, j int) bool { return s[i].GetRegionSize() < s[j].GetRegionSize() }
func (s storeInfo) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (s *balanceRegionScheduler) Schedule(cluster opt.Cluster) *operator.Operator {
	// Your Code Here (3C).
	// 获取所有store 宕机时间不能太大
	stores := make(storeInfo, 0)
	for _, store := range cluster.GetStores() {
		if store.IsUp() && store.DownTime() < cluster.GetMaxStoreDownTime() {
			stores = append(stores, store)
		}
	}

	// 如果只有自己一个 就不用平衡了
	if stores.Len() < 2 {
		return nil
	}
	// 按照 regionSize 排序
	sort.Sort(stores)

	// 寻找 store 来 move 减小负载 从大到小 随机选中一个幸运儿
	var region *core.RegionInfo
	i := stores.Len() - 1
	for ; i > 0; i-- {
		cluster.GetPendingRegionsWithLock(stores[i].GetID(), func(regions core.RegionsContainer) {
			region = regions.RandomRegion(nil, nil)
		})
		if region != nil {
			break
		}
		cluster.GetFollowersWithLock(stores[i].GetID(), func(regions core.RegionsContainer) {
			region = regions.RandomRegion(nil, nil)
		})
		if region != nil {
			break
		}
		cluster.GetLeadersWithLock(stores[i].GetID(), func(regions core.RegionsContainer) {
			region = regions.RandomRegion(nil, nil)
		})
		if region != nil {
			break
		}
	}
	// 如果没有要搬运的（都是空的）
	if region == nil {
		return nil
	}

	// 要搬运的store
	src := stores[i]
	// 获取这个region占用的store的ID
	ids := region.GetStoreIds()
	// 如果小于就maxReplicas就不继续搬运了
	if len(ids) < cluster.GetMaxReplicas() {
		return nil
	}
	// 从小到大 找一个负载小的 并且没被启用的 放进去
	var dst *core.StoreInfo
	for j := 0; j < i; j++ {
		if _, ok := ids[stores[j].GetID()]; !ok {
			dst = stores[j]
			break
		}
	}
	if dst == nil {
		return nil
	}

	// 防止搬运太频繁
	if src.GetRegionSize()-dst.GetRegionSize() < 2*region.GetApproximateSize() {
		return nil
	}

	// 分配peer
	peer, err := cluster.AllocPeer(dst.GetID())
	if err != nil {
		return nil
	}
	descript := fmt.Sprintf("move from %d to %d", src.GetID(), dst.GetID())

	// move
	op, err := operator.CreateMovePeerOperator(descript, cluster, region, operator.OpBalance, src.GetID(), dst.GetID(), peer.GetId())
	if err != nil {
		return nil
	}

	return op

}
