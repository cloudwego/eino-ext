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

	"github.com/google/uuid"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino-ext/devops/internal/model"
)

var _ ContainerService = &containerServiceImpl{}

//go:generate mockgen -source=container.go -destination=../mock/container_mock.go -package=mock ContainerService
type ContainerService interface {
	AddGlobalGraphInfo(graphName string, graphInfo *compose.GraphInfo, graphOpt model.GraphOption) (graphID string, err error)
	AddCustomGraphInfo(graphName string, graphInfo *compose.GraphInfo, graphOpt model.GraphOption) (graphID string, err error)
	GetGraphInfo(graphID string) (graphInfo model.GraphInfo, exist bool)
	ListGraphs() (graphNameToID map[string]string)
	CreateRunnable(graphID, fromNode string) (runnable model.Runnable, err error)
	GetRunnable(graphID, fromNode string) (runnable model.Runnable, exist bool)
	CreateCanvas(graphID string) (canvas model.CanvasInfo, err error)
	GetCanvas(graphID string) (canvas model.CanvasInfo, exist bool)
}

type containerServiceImpl struct {
	mu sync.RWMutex
	// container: GraphID vs GraphContainer
	container map[string]*model.GraphContainer
}

func newContainerService() ContainerService {
	return &containerServiceImpl{
		mu:        sync.RWMutex{},
		container: make(map[string]*model.GraphContainer, 10),
	}
}

func (s *containerServiceImpl) AddGlobalGraphInfo(graphName string, graphInfo *compose.GraphInfo, graphOpt model.GraphOption) (graphID string, err error) {
	if graphInfo == nil {
		return "", fmt.Errorf("graph info is nil")
	}

	return s.addGraphInfo(graphName, &model.GraphInfo{
		Option:    graphOpt,
		GraphInfo: graphInfo,
	}, true)
}

func (s *containerServiceImpl) AddCustomGraphInfo(graphName string, graphInfo *compose.GraphInfo, graphOpt model.GraphOption) (graphID string, err error) {
	if graphInfo == nil {
		return "", fmt.Errorf("graph info is nil")
	}

	return s.addGraphInfo(graphName, &model.GraphInfo{
		Option:    graphOpt,
		GraphInfo: graphInfo,
	}, false)
}

func (s *containerServiceImpl) addGraphInfo(graphName string, graphInfo *model.GraphInfo, isGlobalAdd bool) (graphID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	gid := ""
	for _, c := range s.container {
		if c.GraphName == graphName {
			if isGlobalAdd {
				return c.GraphID, nil
			}
			gid = c.GraphID
			break
		}
	}

	if len(gid) == 0 {
		genID, err := uuid.NewRandom()
		if err != nil {
			return "", err
		}
		gid = genID.String()
	}

	if s.container == nil {
		s.container = make(map[string]*model.GraphContainer, 10)
	}

	s.container[gid] = &model.GraphContainer{
		GraphID:   gid,
		GraphName: graphName,
		GraphInfo: graphInfo,
	}

	return gid, nil
}

func (s *containerServiceImpl) GetGraphInfo(graphID string) (graphInfo model.GraphInfo, exist bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := s.container[graphID]
	if c == nil || c.GraphInfo == nil {
		return graphInfo, false
	}

	return *c.GraphInfo, true
}

func (s *containerServiceImpl) ListGraphs() (graphNameToID map[string]string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	graphNameToID = make(map[string]string, len(s.container))
	for _, c := range s.container {
		graphNameToID[c.GraphName] = c.GraphID
	}

	return graphNameToID
}

func (s *containerServiceImpl) CreateRunnable(graphID, fromNode string) (runnable model.Runnable, err error) {
	s.mu.Lock()
	c := s.container[graphID]
	s.mu.Unlock()
	if c == nil {
		return runnable, fmt.Errorf("must add graph info first")
	}

	graph, err := c.GraphInfo.BuildDevGraph(fromNode)
	if err != nil {
		return runnable, fmt.Errorf("build dev graph failed, err=%w", err)
	}

	runnable, err = graph.Compile(c.GraphInfo.CompileOptions...)
	if err != nil {
		return runnable, fmt.Errorf("compile failed, err=%w", err)
	}

	s.mu.Lock()
	if c.NodesRunnable == nil {
		c.NodesRunnable = make(map[string]*model.Runnable, 10)
	}
	c.NodesRunnable[fromNode] = &runnable
	s.mu.Unlock()

	return runnable, nil
}

func (s *containerServiceImpl) GetRunnable(graphID, fromNode string) (runnable model.Runnable, exist bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := s.container[graphID]
	if c == nil {
		return runnable, false
	}

	r := c.NodesRunnable[fromNode]
	if r == nil {
		return runnable, false
	}

	return *r, true
}

func (s *containerServiceImpl) CreateCanvas(graphID string) (canvasInfo model.CanvasInfo, err error) {
	s.mu.Lock()
	c := s.container[graphID]
	s.mu.Unlock()
	if c == nil {
		return canvasInfo, fmt.Errorf("must add graph first")
	}

	graphInfo := c.GraphInfo
	canvas, err := graphInfo.BuildCanvas()
	if err != nil {
		return canvasInfo, fmt.Errorf("build canvas failed, err=%w", err)
	}
	canvasInfo = model.CanvasInfo{
		Name:    c.GraphName,
		Version: model.CanvasGraphVersionV1,
		Canvas:  canvas,
	}
	c.CanvasInfo = &canvasInfo

	return canvasInfo, nil
}

func (s *containerServiceImpl) GetCanvas(graphID string) (canvasInfo model.CanvasInfo, exist bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := s.container[graphID]
	if c == nil || c.CanvasInfo == nil {
		return canvasInfo, false
	}

	return *c.CanvasInfo, true
}
