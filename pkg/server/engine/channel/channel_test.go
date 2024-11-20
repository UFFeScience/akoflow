package channel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetInstance(t *testing.T) {
	instance1 := GetInstance()
	assert.NotNil(t, instance1, "Expected instance1 to be non-nil")

	instance2 := GetInstance()
	assert.NotNil(t, instance2, "Expected instance2 to be non-nil")

	assert.Equal(t, instance1, instance2, "Expected instance1 and instance2 to be the same")
}

func TestWorfklowChannel(t *testing.T) {
	instance := GetInstance()
	data := DataChannel{
		Namespace: "test",
		Job:       "job1",
		Id:        1,
	}

	instance.WorfklowChannel <- data
	receivedData := <-instance.WorfklowChannel

	assert.Equal(t, data, receivedData, "Expected received data to be equal to sent data")
}
