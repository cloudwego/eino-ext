/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package milvus

import (
	"testing"

	"github.com/cloudwego/eino/components/indexer"
	"github.com/smartystreets/goconvey/convey"
)

func TestWithPartition(t *testing.T) {
	convey.Convey("test WithPartition", t, func() {
		partitionName := "test_partition"
		option := WithPartition(partitionName)
		
		convey.So(option, convey.ShouldNotBeNil)
		
		// 测试选项是否正确设置
		implOpts := indexer.GetImplSpecificOptions(&ImplOptions{}, option)
		
		convey.So(implOpts.Partition, convey.ShouldEqual, partitionName)
	})
}

func TestWithUpsert(t *testing.T) {
	convey.Convey("test WithUpsert", t, func() {
		option := WithUpsert()
		
		convey.So(option, convey.ShouldNotBeNil)
		
		// 测试选项是否正确设置
		implOpts := indexer.GetImplSpecificOptions(&ImplOptions{}, option)
		
		convey.So(implOpts.Upsert, convey.ShouldBeTrue)
	})
}

func TestImplOptions_Integration(t *testing.T) {
	convey.Convey("test ImplOptions integration", t, func() {
		// 测试多个选项组合使用
		partitionName := "integration_partition"
		options := []indexer.Option{
			WithPartition(partitionName),
			WithUpsert(),
		}
		
		implOpts := indexer.GetImplSpecificOptions(&ImplOptions{}, options...)
		
		convey.So(implOpts.Partition, convey.ShouldEqual, partitionName)
		convey.So(implOpts.Upsert, convey.ShouldBeTrue)
	})
}

func TestImplOptions_DefaultValues(t *testing.T) {
	convey.Convey("test ImplOptions default values", t, func() {
		implOpts := &ImplOptions{}
		
		// 测试默认值
		convey.So(implOpts.Partition, convey.ShouldEqual, "")
		convey.So(implOpts.Upsert, convey.ShouldBeFalse)
	})
}