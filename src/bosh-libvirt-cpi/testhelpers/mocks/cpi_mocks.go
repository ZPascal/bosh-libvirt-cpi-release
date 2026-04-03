package mocks

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"
	"github.com/stretchr/testify/mock"

	"bosh-libvirt-cpi/disk"
	"bosh-libvirt-cpi/stemcell"
	"bosh-libvirt-cpi/vm"
)

// Testify mocks for disk operations
type MockDiskCreator struct {
	mock.Mock
}

func (m *MockDiskCreator) Create(size int) (disk.Disk, error) {
	args := m.Called(size)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(disk.Disk), args.Error(1)
}

type MockDiskFinder struct {
	mock.Mock
}

func (m *MockDiskFinder) Find(cid apiv1.DiskCID) (disk.Disk, error) {
	args := m.Called(cid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(disk.Disk), args.Error(1)
}

type MockDisk struct {
	mock.Mock
}

func (m *MockDisk) ID() apiv1.DiskCID {
	args := m.Called()
	return args.Get(0).(apiv1.DiskCID)
}

func (m *MockDisk) Delete() error {
	args := m.Called()
	return args.Error(0)
}

// MockVMFinder is a mock implementation of vm.Finder with testify support
type MockVMFinder struct {
	mock.Mock
	// Fallback for functional style
	FindFunc func(cid apiv1.VMCID) (vm.VM, error)
}

func NewMockVMFinder() *MockVMFinder {
	return &MockVMFinder{
		FindFunc: func(cid apiv1.VMCID) (vm.VM, error) {
			return nil, nil
		},
	}
}

func (m *MockVMFinder) Find(cid apiv1.VMCID) (vm.VM, error) {
	// First try testify mock
	if len(m.ExpectedCalls) > 0 || len(m.Calls) > 0 {
		args := m.Called(cid)
		if args.Get(0) == nil {
			return nil, args.Error(1)
		}
		return args.Get(0).(vm.VM), args.Error(1)
	}
	// Fall back to functional style
	if m.FindFunc != nil {
		return m.FindFunc(cid)
	}
	return nil, nil
}

type MockVM struct {
	mock.Mock
}

func (m *MockVM) ID() apiv1.VMCID {
	args := m.Called()
	return args.Get(0).(apiv1.VMCID)
}

func (m *MockVM) AttachDisk(disk disk.Disk) error {
	args := m.Called(disk)
	return args.Error(0)
}

func (m *MockVM) DetachDisk(diskCID apiv1.DiskCID) error {
	args := m.Called(diskCID)
	return args.Error(0)
}

func (m *MockVM) GetDisks() ([]apiv1.DiskCID, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]apiv1.DiskCID), args.Error(1)
}

// MockStemcellFinder is a mock implementation of stemcell.Finder
type MockStemcellFinder struct {
	FindFunc func(cid apiv1.StemcellCID) (stemcell.Stemcell, error)
}

func NewMockStemcellFinder() *MockStemcellFinder {
	return &MockStemcellFinder{
		FindFunc: func(cid apiv1.StemcellCID) (stemcell.Stemcell, error) {
			return nil, nil
		},
	}
}

func (m *MockStemcellFinder) Find(cid apiv1.StemcellCID) (stemcell.Stemcell, error) {
	if m.FindFunc != nil {
		return m.FindFunc(cid)
	}
	return nil, nil
}

// MockVMCreator is a mock implementation of vm.Creator
type MockVMCreator struct {
	CreateFunc func(agentID apiv1.AgentID, stemcell stemcell.Stemcell,
		cloudProps apiv1.VMCloudProps, networks apiv1.Networks,
		env apiv1.VMEnv) (vm.VM, error)
}

func NewMockVMCreator() *MockVMCreator {
	return &MockVMCreator{
		CreateFunc: func(agentID apiv1.AgentID, stemcell stemcell.Stemcell,
			cloudProps apiv1.VMCloudProps, networks apiv1.Networks,
			env apiv1.VMEnv) (vm.VM, error) {
			return nil, nil
		},
	}
}

func (m *MockVMCreator) Create(agentID apiv1.AgentID, stemcell stemcell.Stemcell,
	cloudProps apiv1.VMCloudProps, networks apiv1.Networks,
	env apiv1.VMEnv) (vm.VM, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(agentID, stemcell, cloudProps, networks, env)
	}
	return nil, nil
}

// MockStemcellCreator is a mock implementation of stemcell.Creator
type MockStemcellCreator struct {
	CreateFunc func(imagePath string, cloudProps apiv1.StemcellCloudProps) (stemcell.Stemcell, error)
}

func NewMockStemcellCreator() *MockStemcellCreator {
	return &MockStemcellCreator{
		CreateFunc: func(imagePath string, cloudProps apiv1.StemcellCloudProps) (stemcell.Stemcell, error) {
			return nil, nil
		},
	}
}

func (m *MockStemcellCreator) Create(imagePath string, cloudProps apiv1.StemcellCloudProps) (stemcell.Stemcell, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(imagePath, cloudProps)
	}
	return nil, nil
}
