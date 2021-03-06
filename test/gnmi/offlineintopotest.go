// Copyright 2019-present Open Networking Foundation.
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
//

package gnmi

import (
	"context"
	"github.com/onosproject/helmit/pkg/helm"
	"github.com/onosproject/onos-api/go/onos/config/change"
	"github.com/onosproject/onos-api/go/onos/config/diags"
	"github.com/onosproject/onos-api/go/onos/topo"
	"github.com/onosproject/onos-config/test/utils/gnmi"
	"github.com/onosproject/onos-config/test/utils/proto"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const (
	offlineInTopoModPath          = "/system/clock/config/timezone-name"
	offlineInTopoModValue         = "Europe/Rome"
	offlineInTopoModDeviceName    = "offline-dev-1"
	offlineInTopoModDeviceVersion = "1.0.0"
	offlineInTopoModDeviceType    = "Devicesim"
)

// TestOfflineDeviceInTopo tests set/query of a single GNMI path to a single device that is in the config but offline
func (s *TestSuite) TestOfflineDeviceInTopo(t *testing.T) {
	t.Skip()
	topoClient, deviceClientError := gnmi.NewTopoClient()
	assert.NotNil(t, topoClient)
	assert.Nil(t, deviceClientError)
	newDevice := &topo.Object{
		ID:   offlineInTopoModDeviceName,
		Type: topo.Object_ENTITY,
		Obj: &topo.Object_Entity{
			Entity: &topo.Entity{
				KindID: offlineInTopoModDeviceType,
			},
		},
	}

	_ = newDevice.SetAspect(&topo.Configurable{
		Type:    offlineInTopoModDeviceType,
		Address: offlineInTopoModDeviceName + ":11161",
		Version: offlineInTopoModDeviceVersion,
		Timeout: uint64((10 * time.Second).Milliseconds()),
	})

	_ = newDevice.SetAspect(&topo.TLSOptions{Plain: true})

	request := &topo.CreateRequest{
		Object: newDevice,
	}
	addResponse, addResponseError := topoClient.Create(context.Background(), request)
	assert.NotNil(t, addResponse)
	assert.Nil(t, addResponseError)

	// Make a GNMI client to use for requests
	gnmiClient := gnmi.GetGNMIClientOrFail(t)

	// Set a value using gNMI client to the offline device
	devicePath := gnmi.GetDevicePathWithValue(offlineInTopoModDeviceName, offlineInTopoModPath, offlineInTopoModValue, proto.StringVal)
	networkChangeID := gnmi.SetGNMIValueOrFail(t, gnmiClient, devicePath, gnmi.NoPaths, gnmi.NoExtensions)

	// Check that the value was set correctly
	gnmi.CheckGNMIValue(t, gnmiClient, devicePath, offlineInTopoModValue, 0, "Query after set returned the wrong value")

	// Check for pending state on the network change
	changeServiceClient, changeServiceClientErr := gnmi.NewChangeServiceClient()
	assert.Nil(t, changeServiceClientErr)
	assert.True(t, changeServiceClient != nil)
	listNetworkChangeRequest := &diags.ListNetworkChangeRequest{
		Subscribe:     true,
		ChangeID:      networkChangeID,
		WithoutReplay: false,
	}
	listNetworkChangesClient, listNetworkChangesClientErr := changeServiceClient.ListNetworkChanges(context.Background(), listNetworkChangeRequest)
	assert.Nil(t, listNetworkChangesClientErr)
	assert.True(t, listNetworkChangesClient != nil)
	networkChangeResponse, networkChangeResponseErr := listNetworkChangesClient.Recv()
	assert.Nil(t, networkChangeResponseErr)
	assert.True(t, networkChangeResponse != nil)
	assert.Equal(t, change.State_PENDING, networkChangeResponse.Change.Status.State)

	// Start the device simulator
	simulator := helm.
		Chart("device-simulator").
		Release(offlineInTopoModDeviceName)
	err := simulator.Install(true)
	assert.NoError(t, err)
	device, err := gnmi.GetDevice(simulator)
	assert.NoError(t, err)
	err = gnmi.AddDeviceToTopo(device)
	assert.NoError(t, err)

	// Wait for config to connect to the device
	gnmi.WaitForDeviceAvailable(t, offlineInTopoModDeviceName, 1*time.Minute)

	// Check that the network change has completed
	gnmi.WaitForNetworkChangeComplete(t, networkChangeID, 10*time.Second)

	// Interrogate the device to check that the value was set properly
	deviceGnmiClient := gnmi.GetDeviceGNMIClientOrFail(t, simulator)
	gnmi.CheckDeviceValue(t, deviceGnmiClient, devicePath, offlineInTopoModValue)

	gnmi.DeleteSimulator(t, simulator)
}
