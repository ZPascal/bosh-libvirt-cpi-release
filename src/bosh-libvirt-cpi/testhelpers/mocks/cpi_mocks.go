package mocks

import (
	apiv1 "github.com/cloudfoundry/bosh-cpi-go/apiv1"

	"bosh-libvirt-cpi/disk"
	"bosh-libvirt-cpi/stemcell"
	"bosh-libvirt-cpi/vm"
)

// MockDiskCreator is a mock implementation of disk.Creator
type MockDiskCreator struct {
	CreateFunc func(size int) (disk.Disk, error)
}

func NewMockDiskCreator() *MockDiskCreator {
	return &MockDiskCreator{
		CreateFunc: func(size int) (disk.Disk, error) {
			return nil, nil
		},
	}
}

func (m *MockDiskCreator) Create(size int) (disk.Disk, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(size)
	}
	return nil, nil
}

// MockDiskFinder is a mock implementation of disk.Finder
type MockDiskFinder struct {
	FindFunc func(cid apiv1.DiskCID) (disk.Disk, error)
}

func NewMockDiskFinder() *MockDiskFinder {
	return &MockDiskFinder{
		FindFunc: func(cid apiv1.DiskCID) (disk.Disk, error) {
			return nil, nil
		},
	}
}

func (m *MockDiskFinder) Find(cid apiv1.DiskCID) (disk.Disk, error) {
	if m.FindFunc != nil {
		return m.FindFunc(cid)
	}
	return nil, nil
}

// MockVMFinder is a mock implementation of vm.Finder
type MockVMFinder struct {
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
	if m.FindFunc != nil {
		return m.FindFunc(cid)
	}
	return nil, nil
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
