package browseruse

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBrowserUse(t *testing.T) {
	ctx := context.Background()
	but, err := NewBrowserUseTool(ctx, nil)
	assert.Nil(t, err)
	defer but.Cleanup()

	url := "https://www.google.com"
	result, err := but.Execute(&Param{
		Action:       ActionOpenTab,
		URL:          &url,
		Index:        nil,
		Text:         nil,
		ScrollAmount: nil,
		TabID:        nil,
		Query:        nil,
		Keys:         nil,
		Seconds:      nil,
	})
	assert.Nil(t, err)
	log.Print(result)
	url = "https://www.baidu.com"
	result, err = but.Execute(&Param{
		Action:       ActionOpenTab,
		URL:          &url,
		Index:        nil,
		Text:         nil,
		ScrollAmount: nil,
		TabID:        nil,
		Query:        nil,
		Keys:         nil,
		Seconds:      nil,
	})
	assert.Nil(t, err)
	log.Print(result)
	tabId := len(but.tabs) - 2
	result, err = but.Execute(&Param{
		Action:       ActionSwitchTab,
		URL:          nil,
		Index:        nil,
		Text:         nil,
		ScrollAmount: nil,
		TabID:        &tabId,
		Query:        nil,
		Keys:         nil,
		Seconds:      nil,
	})
	assert.Nil(t, err)
	log.Print(result)

	state, err := but.GetCurrentState()
	assert.Nil(t, err)
	log.Printf("%+v", *state)
	time.Sleep(10 * time.Second)
}
