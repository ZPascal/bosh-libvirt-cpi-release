package mocks

// FakeStore is a mock implementation of vm.Store for testing
type FakeStore struct {
	data map[string][]byte

	// Callback functions for custom behavior
	PutFunc    func(path string, contents []byte) error
	GetFunc    func(path string) ([]byte, error)
	DeleteFunc func() error
	ListFunc   func() ([]string, error)
}

func NewFakeStore() *FakeStore {
	return &FakeStore{
		data: make(map[string][]byte),
	}
}

func (s *FakeStore) Put(path string, contents []byte) error {
	if s.PutFunc != nil {
		return s.PutFunc(path, contents)
	}
	s.data[path] = contents
	return nil
}

func (s *FakeStore) Get(path string) ([]byte, error) {
	if s.GetFunc != nil {
		return s.GetFunc(path)
	}
	if data, ok := s.data[path]; ok {
		return data, nil
	}
	return nil, nil
}

func (s *FakeStore) Delete() error {
	if s.DeleteFunc != nil {
		return s.DeleteFunc()
	}
	s.data = make(map[string][]byte)
	return nil
}

func (s *FakeStore) List() ([]string, error) {
	if s.ListFunc != nil {
		return s.ListFunc()
	}
	var keys []string
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys, nil
}

