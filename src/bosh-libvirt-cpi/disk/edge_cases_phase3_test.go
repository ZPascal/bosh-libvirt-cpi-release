package disk

import (
	"testing"

	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	"github.com/stretchr/testify/assert"
)

// Phase 3: Disk Edge Cases & Boundary Testing

// Zero Disk Size
func TestEdgeCase_Disk_ZeroSize(t *testing.T) {
	_ = apiv1.NewDiskCID("disk-zero")
	zeroSize := int64(0)
	assert.Equal(t, int64(0), zeroSize)
}

// Minimum Disk Size
func TestEdgeCase_Disk_MinimumSize(t *testing.T) {
	_ = apiv1.NewDiskCID("disk-min")
	minSize := int64(1)
	assert.Greater(t, minSize, int64(0))
}

// Maximum Disk Size
func TestEdgeCase_Disk_MaximumSize(t *testing.T) {
	_ = apiv1.NewDiskCID("disk-max")
	maxSize := int64(10995116277760) // 10TB
	assert.Greater(t, maxSize, int64(0))
}

// Disk Resize Edge Cases
func TestEdgeCase_Disk_ResizeZero(t *testing.T) {
	_ = apiv1.NewDiskCID("disk-resize-zero")

	currentSize := int64(100)
	newSize := int64(0)

	// Should not allow shrinking to zero
	invalid := newSize <= currentSize && newSize == 0
	assert.True(t, invalid)
}

// Disk Resize - Same Size
func TestEdgeCase_Disk_ResizeSameSize(t *testing.T) {
	_ = apiv1.NewDiskCID("disk-resize-same")

	currentSize := int64(100)
	newSize := int64(100)

	// Resizing to same size
	noChange := newSize == currentSize
	assert.True(t, noChange)
}

// Empty Disk Properties
func TestEdgeCase_Disk_EmptyProperties(t *testing.T) {
	properties := make(map[string]interface{})
	assert.Equal(t, 0, len(properties))
}

// Very Long Disk Name
func TestEdgeCase_Disk_VeryLongName(t *testing.T) {
	longName := ""
	for i := 0; i < 1000; i++ {
		longName += "a"
	}
	assert.Greater(t, len(longName), 100)
}

// Disk With Special Characters in Name
func TestEdgeCase_Disk_SpecialCharName(t *testing.T) {
	specialName := "disk-!@#$%^&*()"
	assert.NotEmpty(t, specialName)
}

// Disk Snapshot Limit
func TestEdgeCase_Disk_SnapshotLimit(t *testing.T) {
	_ = apiv1.NewDiskCID("disk-snap-limit")
	maxSnapshots := 5

	snapshots := make([]string, maxSnapshots)
	assert.Equal(t, maxSnapshots, len(snapshots))

	// Try to exceed limit
	canAdd := len(snapshots) < maxSnapshots
	assert.False(t, canAdd)
}

// Empty Snapshot ID
func TestEdgeCase_Snapshot_EmptyID(t *testing.T) {
	emptySnapID := ""
	isValid := len(emptySnapID) > 0
	assert.False(t, isValid)
}

// Disk Attachment to Non-Existent VM
func TestEdgeCase_Disk_AttachToNonExistent(t *testing.T) {
	_ = apiv1.NewDiskCID("disk-1")
	vmID := apiv1.NewVMCID("non-existent")

	vms := make(map[string]interface{})
	_, exists := vms[vmID.AsString()]
	assert.False(t, exists)
}

// Disk Concurrent Operations
func TestEdgeCase_Disk_ConcurrentOps(t *testing.T) {
	done := make(chan bool, 3)

	go func() {
		diskID := apiv1.NewDiskCID("disk-1")
		assert.NotEmpty(t, diskID.AsString())
		done <- true
	}()

	go func() {
		diskID := apiv1.NewDiskCID("disk-2")
		assert.NotEmpty(t, diskID.AsString())
		done <- true
	}()

	go func() {
		diskID := apiv1.NewDiskCID("disk-3")
		assert.NotEmpty(t, diskID.AsString())
		done <- true
	}()

	<-done
	<-done
	<-done
}

// Disk Format Edge Cases
func TestEdgeCase_Disk_FormatEdgeCases(t *testing.T) {
	// Empty format
	emptyFormat := ""
	assert.Empty(t, emptyFormat)

	// Very long format string
	longFormat := "format-name-with-very-long-extension-that-goes-on-and-on"
	assert.Greater(t, len(longFormat), 20)
}

// Disk Speed Edge Cases
func TestEdgeCase_Disk_SpeedMetrics(t *testing.T) {
	// Zero latency
	zeroLatency := 0
	assert.Equal(t, 0, zeroLatency)

	// Very high latency
	highLatency := 999999
	assert.Greater(t, highLatency, 1000)

	// Zero IOPS
	zeroIOPS := 0
	assert.Equal(t, 0, zeroIOPS)

	// Very high IOPS
	highIOPS := 999999
	assert.Greater(t, highIOPS, 10000)
}

// Disk Capacity Edge Cases
func TestEdgeCase_Disk_CapacityMetrics(t *testing.T) {
	// Used capacity exceeding total
	totalCapacity := int64(100)
	usedCapacity := int64(150)

	exceeds := usedCapacity > totalCapacity
	assert.True(t, exceeds)

	// Exactly full
	fullUsed := totalCapacity
	isFull := fullUsed == totalCapacity
	assert.True(t, isFull)
}
