package vm

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestNewVMProps(t *testing.T) {
	t.Run("uses default values", func(t *testing.T) {
		cloudProps := apiv1.NewVMCloudPropsFromMap(map[string]interface{}{})
		props, err := NewVMProps(cloudProps)
		require.NoError(t, err)
		assert.Equal(t, 512, props.Memory)
		assert.Equal(t, 1, props.CPUs)
		assert.Equal(t, 5000, props.EphemeralDisk)
	})
	t.Run("overrides with custom values", func(t *testing.T) {
		cloudProps := apiv1.NewVMCloudPropsFromMap(map[string]interface{}{
			"memory":         2048,
			"cpus":           4,
			"ephemeral_disk": 10000,
		})
		props, err := NewVMProps(cloudProps)
		require.NoError(t, err)
		assert.Equal(t, 2048, props.Memory)
		assert.Equal(t, 4, props.CPUs)
		assert.Equal(t, 10000, props.EphemeralDisk)
	})
}
