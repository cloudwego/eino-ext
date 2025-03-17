/*
 * Copyright 2024 CloudWeGo Authors
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

package service

import (
	"fmt"
	"sync"

	"github.com/matoous/go-nanoid"

	"github.com/cloudwego/eino-ext/devops/internal/model"
	devmodel "github.com/cloudwego/eino-ext/devops/model"
	"github.com/cloudwego/eino/compose"
)

var _ ContainerService = &containerServiceImpl{}

//go:generate mockgen -source=container.go -destination=../mock/container_mock.go -package=mock ContainerService
type ContainerService interface {
	AddGraphInfo(graphName string, graphInfo *compose.GraphInfo) (graphID string, err error)
	ListGraphs() (graphNameToID map[string]string)
	CreateDevGraph(graphID, fromNode string) (devGraph *model.Graph, err error)
	GetDevGraph(graphID, fromNode string) (devGraph *model.Graph, exist bool)
	CreateCanvas(graphID string) (canvas devmodel.CanvasInfo, err error)
	GetCanvas(graphID string) (canvas devmodel.CanvasInfo, exist bool)
}

const maxGraphNum = 100

type containerServiceImpl struct {
	mu sync.RWMutex
	// container: GraphID vs GraphContainer
	container        map[string]*model.GraphContainer
	graphNameCounter map[string]int
	totalGraphNum    int
}

func newContainerService() ContainerService {
	return &containerServiceImpl{
		mu:               sync.RWMutex{},
		container:        make(map[string]*model.GraphContainer, 8),
		graphNameCounter: make(map[string]int, 8),
	}
}

func (s *containerServiceImpl) AddGraphInfo(rootGN string, rootGI *compose.GraphInfo) (graphID string, err error) {
	if rootGI == nil {
		return "", fmt.Errorf("graph info is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.totalGraphNum > maxGraphNum {
		return "", fmt.Errorf("too many graph, max=%d", maxGraphNum)
	}

	newName := rootGN
	cnt := s.graphNameCounter[rootGN]
	if cnt > 0 {
		newName = fmt.Sprintf("%s_%d", newName, cnt)
	}

	if s.container == nil {
		s.container = make(map[string]*model.GraphContainer, 10)
	}

	var add func(pgid, pgn string, pgi *compose.GraphInfo, subGraphNodes map[string]*model.SubGraphNode) (string, error)
	add = func(pgid, pgn string, pgi *compose.GraphInfo, subGraphNodes map[string]*model.SubGraphNode) (string, error) {
		gid := gonanoid.MustID(6)

		for nk, ni := range pgi.Nodes {
			if ni.GraphInfo == nil {
				continue
			}

			_subGraphNodes := make(map[string]*model.SubGraphNode, 10)
			sgn := fmt.Sprintf("%s/%s", pgn, nk)

			sgid, err := add(gid, sgn, ni.GraphInfo, _subGraphNodes)
			if err != nil {
				return "", err
			}

			subGraphNodes[nk] = &model.SubGraphNode{
				ID:            sgid,
				SubGraphNodes: _subGraphNodes,
			}
		}

		cp := make(map[string]*model.SubGraphNode, len(subGraphNodes))
		for k, v := range subGraphNodes {
			cp[k] = v
		}

		s.container[gid] = &model.GraphContainer{
			GraphID: gid,
			Name:    pgn,
			GraphInfo: &model.GraphInfo{
				GraphInfo:     pgi,
				SubGraphNodes: cp,
			},
		}

		return gid, nil
	}

	subGraphNodes := make(map[string]*model.SubGraphNode, 10)
	rootGraphID, err := add("", newName, rootGI, subGraphNodes)
	if err != nil {
		return "", err
	}

	s.totalGraphNum++
	s.graphNameCounter[rootGN]++

	return rootGraphID, nil
}

func (s *containerServiceImpl) ListGraphs() (gnToID map[string]string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	gnToID = make(map[string]string, len(s.container))
	subGraphs := make(map[string]bool, len(s.container))

	for _, c := range s.container {
		if c.GraphInfo == nil {
			continue
		}
		if len(c.GraphInfo.SubGraphNodes) == 0 {
			continue
		}
		for _, sgn := range c.GraphInfo.SubGraphNodes {
			subGraphs[sgn.ID] = true
		}
	}

	for _, c := range s.container {
		if c.GraphInfo == nil {
			continue
		}
		if subGraphs[c.GraphID] {
			continue
		}
		gnToID[c.Name] = c.GraphID
	}

	return gnToID
}

func (s *containerServiceImpl) CreateDevGraph(graphID, fromNode string) (devGraph *model.Graph, err error) {
	s.mu.Lock()
	c := s.container[graphID]
	s.mu.Unlock()
	if c == nil {
		return devGraph, fmt.Errorf("must add graph info first")
	}

	graph, err := model.BuildDevGraph(c.GraphInfo, fromNode)
	if err != nil {
		return devGraph, fmt.Errorf("build dev graph failed, err=%w", err)
	}

	s.mu.Lock()
	if c.NodeGraphs == nil {
		c.NodeGraphs = make(map[string]*model.Graph, 10)
	}
	c.NodeGraphs[fromNode] = graph
	s.mu.Unlock()

	return graph, nil
}

func (s *containerServiceImpl) GetDevGraph(graphID, fromNode string) (devGraph *model.Graph, exist bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := s.container[graphID]
	if c == nil {
		return devGraph, false
	}

	g := c.NodeGraphs[fromNode]
	if g == nil {
		return devGraph, false
	}

	return g, true
}

func (s *containerServiceImpl) CreateCanvas(graphID string) (canvasInfo devmodel.CanvasInfo, err error) {
	s.mu.Lock()
	c := s.container[graphID]
	s.mu.Unlock()
	if c == nil {
		return canvasInfo, fmt.Errorf("must add graph first")
	}

	graphSchema, err := c.GraphInfo.BuildGraphSchema(c.Name, graphID)
	if err != nil {
		return canvasInfo, fmt.Errorf("build canvas failed, err=%w", err)
	}

	canvasInfo = devmodel.CanvasInfo{
		Version:     devmodel.Version,
		GraphSchema: graphSchema,
	}
	c.CanvasInfo = &canvasInfo

	return canvasInfo, nil
}

func (s *containerServiceImpl) GetCanvas(graphID string) (canvasInfo devmodel.CanvasInfo, exist bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := s.container[graphID]
	if c == nil || c.CanvasInfo == nil {
		return canvasInfo, false
	}

	return *c.CanvasInfo, true
}
