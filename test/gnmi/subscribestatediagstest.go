// Copyright 2020-present Open Networking Foundation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package gnmi

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/onosproject/onos-api/go/onos/config/diags"

	"github.com/onosproject/onos-config/pkg/device"
	"github.com/onosproject/onos-config/test/utils/gnmi"
	"github.com/stretchr/testify/assert"
)

const (
	dateTimePath = "/system/state/current-datetime"
)

var (
	previousTimeDiags time.Time
)

func filterResponse(response *diags.OpStateResponse) bool {
	return response.Pathvalue.Path == dateTimePath
}

// TestSubscribeStateDiags tests a stream subscription to updates to a device using the diags API
func (s *TestSuite) TestSubscribeStateDiags(t *testing.T) {
	previousTimeDiags = time.Now().Add(-5 * time.Minute)

	// Bring up a new simulated device
	simulator := gnmi.CreateSimulator(t)
	defer gnmi.DeleteSimulator(t, simulator)

	deviceName := simulator.Name()
	deviceID := device.ID(deviceName)

	// Wait for config to connect to the device
	gnmi.WaitForDeviceAvailable(t, deviceID, time.Minute)

	// Make an opstate diags client
	opstateClient, err := gnmi.NewOpStateDiagsClient()
	assert.NoError(t, err)

	subscribe := &diags.OpStateRequest{
		DeviceId:  deviceName,
		Subscribe: true,
	}

	stream, streamErr := opstateClient.GetOpState(context.Background(), subscribe)
	assert.NoError(t, streamErr)

	i := 1
	for i <= 5 {
		response, responseErr := stream.Recv()
		if responseErr == io.EOF {
			break
		}
		assert.NoError(t, responseErr)
		if filterResponse(response) {
			validateDiagsStateResponse(t, response)
			i++
		}
	}
}

func validateDiagsStateResponse(t *testing.T, resp *diags.OpStateResponse) {
	t.Helper()
	assert.Equal(t, resp.Pathvalue.Path, dateTimePath)

	updatedTimeString := resp.Pathvalue.Value.ValueToString()
	updatedTime, timeParseError := time.Parse("2006-01-02T15:04:05Z-07:00", updatedTimeString)
	assert.NoError(t, timeParseError, "Error parsing time string from path value")
	assert.True(t, !previousTimeDiags.After(updatedTime), "Path time value is not in the future. Req time %v previous time %v", updatedTime, previousTimeDiags)
	previousTimeDiags = updatedTime
}
